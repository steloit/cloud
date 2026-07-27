package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// PrincipalFunc resolves the request's principal (cookie or bearer) — wired
// from the identity service in main.go; a func seam keeps events from
// importing the identity package (which imports events for recording).
type PrincipalFunc func(ctx context.Context, r *http.Request) (session.Principal, bool)

// AuthorizeFunc runs the two-layer check — wired from identity.Authorizer.
type AuthorizeFunc func(ctx context.Context, p session.Principal, perm rbac.Permission, scope rbac.Scope) error

// Streamer serves the x-streamable variant of listEvents. Strict handlers
// buffer their response, so SSE is handled BEFORE the strict server: Intercept
// wraps the mux and claims GET /v1/envs/{env}/events only when the client
// asks for text/event-stream; everything else falls through.
type Streamer struct {
	Q         *store.Queries
	Hub       *Hub
	Envs      EnvResolver
	Principal PrincipalFunc
	Authorize AuthorizeFunc
}

const heartbeatEvery = 15 * time.Second

func (s *Streamer) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			if env, ok := eventsPathEnv(r.URL.Path); ok {
				s.serve(w, r, env)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// eventsPathEnv matches /v1/envs/{env}/events exactly.
func eventsPathEnv(path string) (string, bool) {
	rest, ok := strings.CutPrefix(path, "/v1/envs/")
	if !ok {
		return "", false
	}
	env, ok := strings.CutSuffix(rest, "/events")
	if !ok || env == "" || strings.Contains(env, "/") {
		return "", false
	}
	return env, true
}

func (s *Streamer) serve(w http.ResponseWriter, r *http.Request, envID string) {
	ctx := r.Context()
	// PRINCIPAL FIRST, then the env lookup. Reversed, this leaks env existence to
	// a caller with NO CREDENTIALS AT ALL: a fabricated env answered 404 and a
	// real one answered 401, so anonymous GET was a cheaper oracle than the
	// authenticated one the 404-for-no-standing conversion below was added to
	// close. The JSON half of this same operation checks the principal first,
	// and the two halves must not disagree — that disagreement is exactly what
	// made the conversion necessary in the first place.
	p, ok := s.Principal(ctx, r)
	if !ok {
		problem.Write(w, r, problem.AuthFailed("no credentials", "Sign in or pass a bearer token."))
		return
	}
	orgID, err := s.Envs.OrgForEnv(ctx, envID)
	if err != nil {
		// Distinguish "no such env" from a real failure, as the JSON half does;
		// mapping every error to 404 hides outages as not-founds.
		if !errors.Is(err, ErrEnvNotFound) {
			problem.Write(w, r, problem.Internal(problem.NewEventID()))
			return
		}
		problem.Write(w, r, problem.NotFound("environment"))
		return
	}
	scope := rbac.Scope{OrgID: orgID, EnvID: envID}
	if err := s.Authorize(ctx, p, "observe.read", scope); err != nil {
		// 404, not 403, for a principal with NO STANDING in the org — a
		// non-member (membership:…) or a key scoped elsewhere (key:…). This
		// endpoint is the SSE half of `listEvents`; its JSON half converts the
		// same denials, and an unknown env already 404s eight lines up. Without
		// the same conversion here, `Accept: text/event-stream` is a one-header
		// existence oracle for any env id — and the JSON half's test makes the
		// endpoint look fenced. A member who merely lacks observe.read (role:…)
		// still gets the honest 403.
		var ns interface{ AccessDeniedNoStanding() bool }
		if errors.As(err, &ns) && ns.AccessDeniedNoStanding() {
			problem.Write(w, r, problem.NotFound("environment"))
			return
		}
		problem.Write(w, r, problem.PermissionDenied(err.Error(), "Ask an org admin for observe access."))
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		problem.Write(w, r, problem.Internal(problem.NewEventID()))
		return
	}

	// Resume point precedence: explicit ?cursor= wins (the CLI's channel),
	// then the browser's automatic Last-Event-ID header (EventSource sends it
	// verbatim from the last `id:` on reconnect) — both are opaque cursors, so
	// the same decoder serves both and native EventSource resumes losslessly.
	var cur Cursor
	raw := r.URL.Query().Get("cursor")
	if raw == "" {
		raw = r.Header.Get("Last-Event-ID")
	}
	if raw != "" {
		if cur, err = DecodeCursor(raw); err != nil {
			problem.Write(w, r, problem.ValidationFailed([]problem.FieldError{{Field: "cursor", Detail: "malformed"}}))
			return
		}
	}
	kind := r.URL.Query().Get("kind")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	// `retry:` tells EventSource how long to wait before reconnecting — the
	// state.md SSE-primary posture (2s), so a dropped stream comes back fast.
	_, _ = fmt.Fprint(w, "retry: 2000\n\n")
	flusher.Flush()

	// Subscribe BEFORE replaying so nothing appended mid-replay is missed;
	// replayed keys are tracked and live duplicates below them are skipped.
	live, cancel := s.Hub.Subscribe(orgID)
	defer cancel()

	// Replay in batches until the backlog is drained — a fixed cap would
	// silently gap between the last replayed row and the live feed.
	const batch = 500
	last := cur
	for {
		rows, err := s.Q.ListOrgEventsAsc(ctx, listAscParams(orgID, kind, last, batch))
		if err != nil {
			return // headers are gone; the client reconnects from its cursor
		}
		for _, evt := range rows {
			writeFrame(w, evt)
			last = Cursor{At: evt.At.Time, ID: evt.ID}
		}
		flusher.Flush()
		if len(rows) < batch {
			break
		}
	}

	heartbeat := time.NewTicker(heartbeatEvery)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case evt, open := <-live:
			if !open {
				return
			}
			if kind != "" && evt.Kind != kind {
				continue
			}
			if !after(evt, last) {
				continue // already sent during replay
			}
			writeFrame(w, evt)
			last = Cursor{At: evt.At.Time, ID: evt.ID}
			flusher.Flush()
		}
	}
}

func after(evt store.Event, c Cursor) bool {
	if c.ID == "" {
		return true
	}
	if evt.At.Time.After(c.At) {
		return true
	}
	return evt.At.Time.Equal(c.At) && evt.ID > c.ID
}

func listAscParams(orgID, kind string, cur Cursor, limit int32) store.ListOrgEventsAscParams {
	p := store.ListOrgEventsAscParams{OrgID: orgID, Limit: limit}
	if kind != "" {
		p.Kind = textOf(kind)
	}
	if cur.ID != "" {
		p.AfterAt = tstzOf(cur.At)
		p.AfterID = textOf(cur.ID)
	}
	return p
}

// writeFrame emits one SSE frame; `id:` is the event's cursor, so the
// browser's automatic Last-Event-ID (or an explicit ?cursor=) resumes with no
// gap and no duplicate.
func writeFrame(w http.ResponseWriter, evt store.Event) {
	body, err := json.Marshal(ToAPI(evt))
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", EncodeCursor(evt.At.Time, evt.ID), evt.Kind, body)
}

func textOf(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

func tstzOf(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

// ToAPI maps a ledger row onto the contract Event schema.
func ToAPI(evt store.Event) gen.Event {
	out := gen.Event{
		Id:    evt.ID,
		Kind:  evt.Kind,
		At:    evt.At.Time,
		Actor: evt.Actor,
	}
	via := gen.EventVia(evt.Via)
	out.Via = &via
	if evt.Action != "" {
		a := evt.Action
		out.Action = &a
	}
	if evt.Subject != "" {
		s := evt.Subject
		out.Subject = &s
	}
	var detail map[string]interface{}
	if len(evt.Detail) > 0 && json.Unmarshal(evt.Detail, &detail) == nil {
		out.Detail = &detail
	}
	return out
}
