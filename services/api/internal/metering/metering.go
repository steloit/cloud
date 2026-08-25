// Package metering is the D10 raw usage store: every billing-relevant
// lifecycle edge emits, from the FIRST resource — backfill is impossible, so
// this shipped with the first status machine, not after it. The control
// plane emits span edges (open at ready, close when billing stops); the
// cell-agent adds fine-grained meters (compute-seconds, egress) when it
// lands; T6.3's rollup turns both into quota_usage.
package metering

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
)

// Tags locate a usage event forever — they survive resource deletion (no
// FKs by design: billing truth outlives the resource tree).
type Tags struct {
	OrgID, ProjectID, EnvID, ServiceID string
}

type Emitter struct{ q *store.Queries }

func NewEmitter(q *store.Queries) *Emitter { return &Emitter{q: q} }

// SpanKeyForStatus is the dedupe key for a span edge caused by a STATUS
// transition. Derived from committed state — `status_changed_at` comes off the
// row the transition just wrote — so a retry of the same edge computes the same
// key and a genuinely later transition computes a different one.
//
// It must not be built from `time.Now()` or a fresh id: those make every retry a
// new event, which is the behaviour the key exists to prevent.
func SpanKeyForStatus(serviceID, edge string, statusChangedAt time.Time) string {
	return fmt.Sprintf("%s:%s:%d", serviceID, edge, statusChangedAt.UTC().UnixNano())
}

// SpanKeyForReprice is the dedupe key for the close/open pair a price change
// emits. A reprice is a desired-state write, so `generation` identifies it; the
// pair shares a generation and is separated by `edge`.
func SpanKeyForReprice(serviceID, edge string, generation int64) string {
	return fmt.Sprintf("%s:%s:reprice:%d", serviceID, edge, generation)
}

// EmitSpan records a billing span edge for a service. meter service_span:
// open at `ready` (metering starts at ready — ADR-024), close when billing
// stops (suspended, deleting, failed-from-degraded). rateCents snapshots the
// monthly estimate at the edge so later price changes never rewrite history.
//
// IDEMPOTENT ON dedupeKey (O38). Emitting the same edge twice writes one row and
// the second call succeeds — a caller retrying is behaving correctly, and the
// whole reason to have the key is that BEFORE it, retrying was unsafe, so we did
// not retry, so a failed emit was a silent billing gap with a log line for a
// recovery path. `duplicate` reports which happened, for callers that want to say
// so; neither is an error.
func (e *Emitter) EmitSpan(ctx context.Context, tags Tags, dedupeKey, edge, product string, rateCents int64) (duplicate bool, err error) {
	if tags.OrgID == "" || tags.ProjectID == "" || tags.EnvID == "" || tags.ServiceID == "" {
		return false, fmt.Errorf("metering: all tags are required (org/project/env/service)")
	}
	if dedupeKey == "" {
		// Fail rather than fall back to a random key: a random one satisfies the
		// column and defeats the mechanism, which is worse than refusing because
		// it looks like it worked.
		return false, fmt.Errorf("metering: dedupe key is required — derive it from committed state (SpanKeyFor*), never at random")
	}
	n, err := e.q.InsertUsageEvent(ctx, store.InsertUsageEventParams{
		ID: ids.New("use"), DedupeKey: dedupeKey, OrgID: tags.OrgID, ProjectID: tags.ProjectID,
		EnvID: tags.EnvID, ServiceID: tags.ServiceID,
		Meter: "service_span", Edge: edge, Product: product,
		RateCents: rateCents, Quantity: 0, Detail: []byte("{}"),
	})
	if err != nil {
		return false, fmt.Errorf("metering: emit: %w", err)
	}
	return n == 0, nil
}

// MustEmitSpan is EmitSpan for post-commit paths: a metering failure after a
// committed state change is logged LOUDLY (it is a billing incident, D10)
// but never rolls back the change it describes.
func (e *Emitter) MustEmitSpan(ctx context.Context, tags Tags, dedupeKey, edge, product string, rateCents int64) {
	if _, err := e.EmitSpan(ctx, tags, dedupeKey, edge, product, rateCents); err != nil {
		slog.Error("METERING EMIT FAILED — billing gap, investigate now (D10)",
			"service", tags.ServiceID, "edge", edge, "dedupe_key", dedupeKey, "err", err)
	}
}

// IsBilling reports whether a status has an OPEN billing span. A rate change
// while one is open is invisible to the invoice unless the span is closed and
// reopened — rate_cents is snapshotted at open and the rollup weights every
// second of the span at that snapshot.
func IsBilling(status string) bool { return billingStates[status] }

var billingStates = map[string]bool{"ready": true, "degraded": true}

// BillingEdge classifies a status transition for the span machine:
// "open" when billing starts, "close" when it stops, "" when unaffected.
// degraded still bills (the resource runs); suspended/deleting/failed stop.
func BillingEdge(from, to string) string {
	billing := billingStates
	switch {
	case !billing[from] && billing[to]:
		return "open"
	case billing[from] && !billing[to]:
		return "close"
	default:
		return ""
	}
}
