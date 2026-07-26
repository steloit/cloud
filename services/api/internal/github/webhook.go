// Package github is the T4.1 integration ingress: ONE org-level GitHub App
// (G3), push + PR webhooks received, verified, and stored idempotently.
// This is a machine-integration endpoint, not customer surface — it lives
// outside the /v1 contract (GitHub cannot speak problem+json), and no
// substrate grammar leaks from it (D8). The build pipeline (T4.2) and
// preview orchestration (T4.4) consume stored deliveries when they land.
package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
)

const maxPayload = 1 << 20 // 1 MiB — GitHub's own cap is 25 MB; we store essentials

type Handler struct {
	q      *store.Queries
	rec    *events.Recorder
	secret []byte // GITHUB_WEBHOOK_SECRET; empty = integration not configured
}

func NewHandler(q *store.Queries, rec *events.Recorder, secret string) *Handler {
	return &Handler{q: q, rec: rec, secret: []byte(secret)}
}

// Mount registers the ingress. Kept off /v1 deliberately (integration, not
// contract).
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /integrations/github/webhook", h.serve)
}

// verify checks X-Hub-Signature-256 (HMAC-SHA256, constant-time).
func (h *Handler) verify(sig string, body []byte) bool {
	const prefix = "sha256="
	if len(sig) <= len(prefix) || sig[:len(prefix)] != prefix {
		return false
	}
	want, err := hex.DecodeString(sig[len(prefix):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, h.secret)
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), want)
}

type pushPayload struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
}

type prPayload struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest struct {
		Head struct {
			Sha string `json:"sha"`
			Ref string `json:"ref"`
		} `json:"head"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
}

type installationPayload struct {
	Action       string `json:"action"`
	Installation struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
		} `json:"account"`
	} `json:"installation"`
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	if len(h.secret) == 0 {
		// Pre-P2 honesty: the app isn't registered yet; refuse loudly.
		http.Error(w, "github integration not configured", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxPayload+1))
	if err != nil || len(body) > maxPayload {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	if !h.verify(r.Header.Get("X-Hub-Signature-256"), body) {
		http.Error(w, "signature mismatch", http.StatusUnauthorized)
		return
	}
	event := r.Header.Get("X-GitHub-Event")
	deliveryID := r.Header.Get("X-GitHub-Delivery")
	if event == "" || deliveryID == "" {
		http.Error(w, "missing event headers", http.StatusBadRequest)
		return
	}

	action, repo := "", ""
	switch event {
	case "push":
		var p pushPayload
		_ = json.Unmarshal(body, &p)
		repo = p.Repository.FullName
	case "pull_request":
		var p prPayload
		_ = json.Unmarshal(body, &p)
		action, repo = p.Action, p.Repository.FullName
	case "installation":
		var p installationPayload
		_ = json.Unmarshal(body, &p)
		action = p.Action
	}

	stored, err := h.q.InsertDelivery(r.Context(), store.InsertDeliveryParams{
		ID: ids.New("ghd"), DeliveryID: deliveryID, Event: event, Action: action,
		Repo: repo, Payload: body,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// duplicate delivery id: already processed — idempotent 200
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "duplicate delivery ignored")
		return
	}
	if err != nil {
		http.Error(w, "storage failure", http.StatusInternalServerError)
		return
	}
	_ = stored

	switch event {
	case "installation":
		h.handleInstallation(r, body)
	case "push":
		h.handlePush(r, body)
	case "pull_request":
		h.handlePR(r, body)
	}
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintln(w, "stored")
}

// handleInstallation — created keeps the row alive; deleted marks it. The
// org mapping for a NEW installation needs the setup flow (the console's G3
// screen posts it, E8): until then created installs are stored as deliveries
// only, and the finding is recorded on the task.
func (h *Handler) handleInstallation(r *http.Request, body []byte) {
	var p installationPayload
	if json.Unmarshal(body, &p) != nil {
		return
	}
	if p.Action == "deleted" {
		_, _ = h.q.MarkInstallationDeleted(r.Context(), p.Installation.ID)
	}
}

// handlePush — a push on a linked repo lands a spine event per linked
// service (the T4.2 build pipeline consumes these when it exists).
func (h *Handler) handlePush(r *http.Request, body []byte) {
	var p pushPayload
	if json.Unmarshal(body, &p) != nil || p.Repository.FullName == "" {
		return
	}
	orgID, err := h.q.OrgForInstallation(r.Context(), p.Installation.ID)
	if err != nil {
		return // unlinked installation: delivery stored, nothing to mark
	}
	links, err := h.q.LinksForRepo(r.Context(), store.LinksForRepoParams{OrgID: orgID, Repo: p.Repository.FullName})
	if err != nil {
		return
	}
	for _, link := range links {
		if "refs/heads/"+link.Branch != p.Ref {
			continue
		}
		_, _ = h.rec.Append(r.Context(), events.Input{
			OrgID: orgID, Kind: "deploy", Via: "system", Actor: "github",
			Action: "git.push", Subject: link.ServiceID,
			Detail: []byte(`{"repo":` + strconv.Quote(p.Repository.FullName) + `,"sha":` + strconv.Quote(p.After) + `,"ref":` + strconv.Quote(p.Ref) + `}`),
		})
	}
}

// handlePR — opened/closed on a linked repo lands spine events (preview
// orchestration T4.4 consumes them).
func (h *Handler) handlePR(r *http.Request, body []byte) {
	var p prPayload
	if json.Unmarshal(body, &p) != nil || p.Repository.FullName == "" {
		return
	}
	if p.Action != "opened" && p.Action != "closed" && p.Action != "reopened" {
		return
	}
	orgID, err := h.q.OrgForInstallation(r.Context(), p.Installation.ID)
	if err != nil {
		return
	}
	links, err := h.q.LinksForRepo(r.Context(), store.LinksForRepoParams{OrgID: orgID, Repo: p.Repository.FullName})
	if err != nil || len(links) == 0 {
		return
	}
	_, _ = h.rec.Append(r.Context(), events.Input{
		OrgID: orgID, Kind: "deploy", Via: "system", Actor: "github",
		Action: "git.pr_" + p.Action, Subject: links[0].ServiceID,
		Detail: []byte(`{"repo":` + strconv.Quote(p.Repository.FullName) + `,"number":` + strconv.Itoa(p.Number) + `,"sha":` + strconv.Quote(p.PullRequest.Head.Sha) + `}`),
	})
}
