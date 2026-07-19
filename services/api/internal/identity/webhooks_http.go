package identity

// T10.3 org webhooks (governance): create with a reveal-once signing secret,
// list, and a synchronous test that returns its result inline (U8 — never a
// dead end). Webhooks are the org's outbound robots — the same integration-
// credential class as org API keys, so they carry the api_keys.manage
// permission (a dedicated webhooks.manage is a proposed refinement, see the
// spec-change-proposals ledger).

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/notify"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

const webhookPerm = "api_keys.manage"

func (h *Handlers) ListWebhooks(ctx context.Context, req gen.ListWebhooksRequestObject) (gen.ListWebhooksResponseObject, error) {
	if _, err := h.requireOrg(ctx, req.OrgPathParam, webhookPerm, false); err != nil {
		return nil, err
	}
	rows, err := h.svc.q.ListWebhooks(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	out := make([]gen.Webhook, 0, len(rows))
	for _, w := range rows {
		out = append(out, webhookToAPI(w))
	}
	return gen.ListWebhooks200JSONResponse(gen.WebhookList{Data: &out}), nil
}

func (h *Handlers) CreateWebhook(ctx context.Context, req gen.CreateWebhookRequestObject) (gen.CreateWebhookResponseObject, error) {
	actor, err := h.requireOrg(ctx, req.OrgPathParam, webhookPerm, true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || req.Body.Url == "" {
		return nil, validationError{fields: []problem.FieldError{{Field: "url", Detail: "required"}}}
	}
	// SSRF gate at create (delivery re-checks): https + a routable host only.
	if err := notify.ValidateURL(req.Body.Url); err != nil {
		return nil, validationError{fields: []problem.FieldError{{Field: "url", Detail: err.Error()}}}
	}
	id := ids.New("wbh")
	secret, sealed, err := h.notify.MintSecret(id)
	if err != nil {
		return nil, err
	}
	kinds := req.Body.Events
	if kinds == nil {
		kinds = []string{}
	}
	row, err := h.svc.q.CreateWebhook(ctx, store.CreateWebhookParams{
		ID: id, OrgID: req.OrgPathParam, Url: req.Body.Url, Events: kinds,
		Ciphertext: sealed.Ciphertext, Nonce: sealed.Nonce,
		WrappedDek: sealed.WrappedDEK, DekNonce: sealed.DEKNonce, KekID: sealed.KEKID,
	})
	if err != nil {
		return nil, err
	}
	h.svc.record(ctx, events.Input{
		OrgID: req.OrgPathParam, Kind: "lifecycle", Via: "user", Actor: actor,
		Action: "webhook.created", Subject: row.ID,
		Detail: []byte(`{"url":` + strconv.Quote(row.Url) + `}`),
	})
	created := gen.WebhookCreated{
		Id: row.ID, Url: row.Url, Events: row.Events,
		Status: gen.WebhookCreatedStatus(row.Status), Secret: &secret,
	}
	if row.CreatedAt.Valid {
		t := row.CreatedAt.Time
		created.CreatedAt = &t
	}
	return gen.CreateWebhook201JSONResponse(created), nil
}

// MountWebhookTest registers the testWebhook route MANUALLY, pre-strict: the
// contract path `/webhooks/{wbh}:test` glues a `:verb` custom-method onto a
// path parameter, which Go's stdlib ServeMux cannot parse (a wildcard segment
// must end with `}`). It is the only such path in the spec; the SSE streamer
// established this pre-strict pattern. (Finding filed: the `{param}:verb` shape
// needs either this shim or a `/webhooks/{wbh}/test` restructure.)
func (h *Handlers) MountWebhookTest(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/webhooks/", func(w http.ResponseWriter, r *http.Request) {
		if err := h.testWebhook(r.Context(), w, r); err != nil {
			h.responseError(w, r, err)
		}
	})
}

func (h *Handlers) testWebhook(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	rest := strings.TrimPrefix(r.URL.Path, "/v1/webhooks/")
	wbh, ok := strings.CutSuffix(rest, ":test")
	if !ok || wbh == "" || strings.ContainsAny(wbh, "/:") {
		return notFoundError{what: "webhook"}
	}
	p, ok := h.svc.PrincipalFromRequest(ctx, r)
	if !ok {
		return ErrNoSession
	}
	wh, err := h.svc.q.GetWebhook(ctx, wbh)
	if err != nil {
		return notFoundError{what: "webhook"}
	}
	// gate on the webhook's OWN org so a caller can't test one they don't
	// administer (mutating: a read_only token may not fire a delivery).
	if p.Kind == "token" && p.Scope != "full" {
		return ErrScopeDenied
	}
	if err := h.authz.Require(ctx, p, webhookPerm, rbac.Scope{OrgID: wh.OrgID}); err != nil {
		return err
	}
	res, err := h.notify.Test(ctx, wbh)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(map[string]any{
		"delivered": res.Delivered, "status_code": res.StatusCode, "duration_ms": res.DurationMs,
	})
}

func webhookToAPI(w store.Webhook) gen.Webhook {
	out := gen.Webhook{Id: w.ID, Url: w.Url, Events: w.Events, Status: gen.WebhookStatus(w.Status)}
	if out.Events == nil {
		out.Events = []string{}
	}
	if w.CreatedAt.Valid {
		t := w.CreatedAt.Time
		out.CreatedAt = &t
	}
	return out
}
