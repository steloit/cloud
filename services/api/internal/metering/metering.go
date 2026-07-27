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

// EmitSpan records a billing span edge for a service. meter service_span:
// open at `ready` (metering starts at ready — ADR-024), close when billing
// stops (suspended, deleting, failed-from-degraded). rateCents snapshots the
// monthly estimate at the edge so later price changes never rewrite history.
func (e *Emitter) EmitSpan(ctx context.Context, tags Tags, edge, product string, rateCents int64) error {
	if tags.OrgID == "" || tags.ProjectID == "" || tags.EnvID == "" || tags.ServiceID == "" {
		return fmt.Errorf("metering: all tags are required (org/project/env/service)")
	}
	_, err := e.q.InsertUsageEvent(ctx, store.InsertUsageEventParams{
		ID: ids.New("use"), OrgID: tags.OrgID, ProjectID: tags.ProjectID,
		EnvID: tags.EnvID, ServiceID: tags.ServiceID,
		Meter: "service_span", Edge: edge, Product: product,
		RateCents: rateCents, Quantity: 0, Detail: []byte("{}"),
	})
	if err != nil {
		return fmt.Errorf("metering: emit: %w", err)
	}
	return nil
}

// MustEmitSpan is EmitSpan for post-commit paths: a metering failure after a
// committed state change is logged LOUDLY (it is a billing incident, D10)
// but never rolls back the change it describes.
func (e *Emitter) MustEmitSpan(ctx context.Context, tags Tags, edge, product string, rateCents int64) {
	if err := e.EmitSpan(ctx, tags, edge, product, rateCents); err != nil {
		slog.Error("METERING EMIT FAILED — billing gap, investigate now (D10)",
			"service", tags.ServiceID, "edge", edge, "err", err)
	}
}

// BillingEdge classifies a status transition for the span machine:
// "open" when billing starts, "close" when it stops, "" when unaffected.
// degraded still bills (the resource runs); suspended/deleting/failed stop.
// IsBilling reports whether a status has an OPEN billing span. A rate change
// while one is open is invisible to the invoice unless the span is closed and
// reopened — rate_cents is snapshotted at open and the rollup weights every
// second of the span at that snapshot.
func IsBilling(status string) bool { return billingStates[status] }

var billingStates = map[string]bool{"ready": true, "degraded": true}

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
