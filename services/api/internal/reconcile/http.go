package reconcile

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// Handlers mounts the two internal-plane endpoints PRE-STRICT, the same
// precedent as testWebhook and the billing CSV exports. Deliberate, and a
// recorded deviation from the resume plan's "register the op ids" step:
// the strict server's contextMiddleware resolves USER principals (cookies,
// personal tokens, org keys) into ctx — the reconcile plane must never pass
// through that resolver, and strict handlers do not expose the raw
// Authorization header a separate principal needs. Pre-strict keeps the two
// principals structurally apart; the ops therefore also stay OUT of
// include-operation-ids, exactly as testWebhook does (the contract in
// openapi.yaml is unchanged — this is about which server stack serves it).
type Handlers struct {
	svc  *Service
	auth *Auth
}

func NewHandlers(svc *Service, auth *Auth) *Handlers { return &Handlers{svc: svc, auth: auth} }

// Mount registers the reconcile plane. With no secret configured the routes
// are still registered but answer 503 — visibly closed, never silently open.
func (h *Handlers) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/reconcile/{cell}/desired", h.desired)
	mux.HandleFunc("POST /v1/reconcile/{cell}/status", h.status)
}

// gate runs the auth ladder shared by both endpoints:
// 503 unconfigured → 401 bad token → 404 not-your-cell (never 403 — a
// reconciler token must not learn which other cells exist).
func (h *Handlers) gate(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !h.auth.Enabled() {
		problem.Write(w, r, problem.Problem{
			Type: "https://api.steloit.com/errors/unavailable", Title: "Reconciler disabled",
			Status:      http.StatusServiceUnavailable,
			Detail:      "No reconciler secret is configured on this control plane.",
			Remediation: "Set RECONCILER_SECRET (and RECONCILER_CELLS) on the api service.",
		})
		return "", false
	}
	if !h.auth.Authenticated(r) {
		problem.Write(w, r, problem.AuthFailed(
			"This endpoint accepts reconciler-scoped tokens only.",
			"Use the cell-agent's reconciler token; user sessions and org API keys are not valid here."))
		return "", false
	}
	cell := r.PathValue("cell")
	if !h.auth.Allows(r, cell) {
		problem.Write(w, r, problem.NotFound("cell"))
		return "", false
	}
	return cell, true
}

func (h *Handlers) desired(w http.ResponseWriter, r *http.Request) {
	cell, ok := h.gate(w, r)
	if !ok {
		return
	}
	var since int64
	if q := r.URL.Query().Get("since_generation"); q != "" {
		v, err := strconv.ParseInt(q, 10, 64)
		if err != nil {
			problem.Write(w, r, problem.ValidationFailed([]problem.FieldError{
				{Field: "since_generation", Detail: "must be an integer"}}))
			return
		}
		since = v
	}
	var limit int64
	if q := r.URL.Query().Get("limit"); q != "" {
		v, err := strconv.ParseInt(q, 10, 32)
		if err != nil || v < 1 {
			problem.Write(w, r, problem.ValidationFailed([]problem.FieldError{
				{Field: "limit", Detail: "must be a positive integer"}}))
			return
		}
		limit = v
	}
	services, err := h.svc.Desired(r.Context(), cell, since, int32(limit))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": services})
}

// statusBody mirrors the contract's request schema. Decoded strictly: an
// unknown field is a defect in the agent, not something to ignore quietly.
//
// Conditions is RESERVED (US-1.3): it is decoded so the wire format is stable,
// validated for shape by the strict decoder, and acknowledged — but not yet
// persisted (no column). Condition storage is a follow-up; accepting the field
// now means the agent's writeback format does not change when it lands.
type statusBody struct {
	ServiceID          string          `json:"service_id"`
	ObservedGeneration *int64          `json:"observed_generation"`
	Status             string          `json:"status"`
	Conditions         json.RawMessage `json:"conditions"` // reserved; see above
	Event              string          `json:"event"`
}

var statusVocab = map[string]bool{
	"provisioning": true, "ready": true, "degraded": true, "failed": true,
	"suspended": true, "deleting": true, "gone": true, "": true,
}

func (h *Handlers) status(w http.ResponseWriter, r *http.Request) {
	cell, ok := h.gate(w, r)
	if !ok {
		return
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	var b statusBody
	if err := dec.Decode(&b); err != nil {
		problem.Write(w, r, problem.ValidationFailed([]problem.FieldError{
			{Field: "body", Detail: "invalid JSON: " + err.Error()}}))
		return
	}
	var fields []problem.FieldError
	if b.ServiceID == "" {
		fields = append(fields, problem.FieldError{Field: "service_id", Detail: "required"})
	}
	if b.ObservedGeneration == nil {
		fields = append(fields, problem.FieldError{Field: "observed_generation", Detail: "required"})
	} else if *b.ObservedGeneration < 0 {
		fields = append(fields, problem.FieldError{Field: "observed_generation", Detail: "must be >= 0"})
	}
	if !statusVocab[b.Status] {
		fields = append(fields, problem.FieldError{Field: "status", Detail: "not in the ADR-024 vocabulary"})
	}
	if len(fields) > 0 {
		problem.Write(w, r, problem.ValidationFailed(fields))
		return
	}
	// "gone" reports a completed teardown; the row's terminal state is
	// `deleting` (ADR-024) and row removal is the deletion pipeline's job
	// (US-3.5's final-backup contract), so it maps to observation-only here.
	st := b.Status
	if st == "gone" {
		st = ""
	}
	svc, err := h.svc.Writeback(r.Context(), cell, Report{
		ServiceID: b.ServiceID, ObservedGeneration: *b.ObservedGeneration,
		Status: st, Conditions: b.Conditions, Event: b.Event,
	})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service_id": svc.ID, "status": svc.Status, "observed_generation": svc.ObservedGeneration,
	})
}

func (h *Handlers) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrUnknownCell):
		problem.Write(w, r, problem.NotFound("cell or service"))
	case errors.Is(err, ErrStaleGeneration):
		problem.Write(w, r, problem.Conflict(
			[]string{"the reported generation is not the one desired currently holds"},
			"Re-poll /desired and report on the current generation."))
	// 409, NOT 500. An unconverged hop is a NORMAL, expected event — the status
	// machine took a legal edge and needs one more converge to finish (today only
	// `failed` + a healthy cluster, which ADR-024 routes through `provisioning`).
	// Without this arm it fell to `default:` and became `problem.Internal`: an
	// HTTP 500 the OpenAPI contract does not declare for this route, an ERROR
	// boundary log, and a remediation telling the operator to "contact support
	// with the event id" — an id that is never minted. Every failed-service
	// recovery in the fleet would emit a control-plane 5xx.
	//
	// 409 is already in the contract and is already the right shape for the
	// agent: its own client doc says a 409 is "just another re-poll — the row
	// stays outstanding server-side, so the next tick re-converges it".
	case errors.Is(err, ErrNotConverged):
		problem.Write(w, r, problem.Conflict(
			[]string{"the service needs one more converge to finish this generation"},
			"Re-poll /desired and report again — the edge was taken and the row is still outstanding."))
	default:
		if c, ok := errors.AsType[problem.Carrier](err); ok {
			problem.Write(w, r, c.Problem()) // e.g. Transition's illegal-edge 409
			return
		}
		problem.Write(w, r, problem.Internal(""))
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
