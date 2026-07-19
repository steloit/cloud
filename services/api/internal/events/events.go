// Package events is the spine (GOV-002 primitive 9): ONE append-only pipeline
// feeding everything that "happened" — audit, /events, SSE, the bell,
// notifications, deploy markers. There is exactly one events table;
// /orgs/{org}/audit and /envs/{env}/events are views over it. Corrections are
// new events; `via` provenance (user | assistant | system) is forever.
package events

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
)

// Input is one fact to append. Kind/action come from the module's closed
// vocabulary, never from user input.
type Input struct {
	OrgID   string
	Kind    string // deploy | scale | alert_state | policy_trigger | lifecycle | membership | billing | …
	Via     string // user | assistant | system
	Actor   string // usr_/tok_ id or "system"
	Action  string // e.g. org.created, member.added
	Subject string // the xxx_ id acted on
	Detail  []byte // jsonb; nil → {}
}

// Recorder appends to the ledger and fans out to live subscribers — in that
// order: publish only after the row committed.
type Recorder struct {
	q   *store.Queries
	hub *Hub
}

func NewRecorder(q *store.Queries, hub *Hub) *Recorder {
	return &Recorder{q: q, hub: hub}
}

func (r *Recorder) Append(ctx context.Context, in Input) (store.Event, error) {
	detail := in.Detail
	if detail == nil {
		detail = []byte("{}")
	}
	evt, err := r.q.AppendEvent(ctx, store.AppendEventParams{
		ID: ids.New("evt"), OrgID: in.OrgID, Kind: in.Kind, Via: in.Via,
		Actor: in.Actor, Action: in.Action, Subject: in.Subject, Detail: detail,
	})
	if err != nil {
		return store.Event{}, fmt.Errorf("events: append: %w", err)
	}
	r.hub.Publish(evt)
	return evt, nil
}

// ---- cursor -----------------------------------------------------------------

// Cursor is a keyset position over (at, id) — offset pagination drifts under
// append. Encoded opaque; a cursor from a foreign org simply matches nothing
// because every query is org-fenced.
type Cursor struct {
	At time.Time
	ID string
}

func EncodeCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func DecodeCursor(s string) (Cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, fmt.Errorf("events: bad cursor")
	}
	at, id, ok := strings.Cut(string(raw), "|")
	if !ok {
		return Cursor{}, fmt.Errorf("events: bad cursor")
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return Cursor{}, fmt.Errorf("events: bad cursor")
	}
	return Cursor{At: t, ID: id}, nil
}

// EnvResolver maps an environment id to its owning org for scoping /envs/
// reads. Real environments arrive with T3.2; until then the only production
// implementation is NoEnvs and tests inject fakes. This seam is the T2.5
// "skeleton" boundary — recorded, not silent.
type EnvResolver interface {
	OrgForEnv(ctx context.Context, envID string) (orgID string, err error)
}

// ErrEnvNotFound is returned by resolvers for unknown environments.
var ErrEnvNotFound = fmt.Errorf("events: environment not found")

// NoEnvs is the pre-T3.2 resolver: no environments exist yet.
type NoEnvs struct{}

func (NoEnvs) OrgForEnv(context.Context, string) (string, error) { return "", ErrEnvNotFound }
