package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/steloit/cloud/services/api/internal/identity/store"
)

// Querier is the store surface this package needs. Narrow on purpose: the
// reconciler reads desired state and records observation — it must not be able
// to reach the provisioning writers, so the interface simply does not offer
// them.
type Querier interface {
	GetCell(ctx context.Context, id string) (store.Cell, error)
	ListDesiredForCell(ctx context.Context, arg store.ListDesiredForCellParams) ([]store.ListDesiredForCellRow, error)
	MarkObserved(ctx context.Context, arg store.MarkObservedParams) (store.Service, error)
	TouchCellHeartbeat(ctx context.Context, id string) error
	GetService(ctx context.Context, id string) (store.Service, error)
	OrgForService(ctx context.Context, id string) (string, error)
}

// Transitioner is the EXISTING guarded status machine
// (provisioning.Service.Transition). The reconciler does not reimplement it:
// legal edges (ADR-024), spine events on every edge (D10), and the
// metering-starts-at-ready rule all live there, and a second copy would drift
// from the first. This package only decides *when* to call it.
type Transitioner interface {
	Transition(ctx context.Context, svc store.Service, to, via, actor, orgID string) (store.Service, error)
}

// Service is the control-plane half of the reconciler protocol.
type Service struct {
	q     Querier
	trans Transitioner
}

func New(q Querier, trans Transitioner) *Service { return &Service{q: q, trans: trans} }

// ErrStaleGeneration is a writeback whose reported generation is not the one
// desired holds right now — either BEHIND (the agent converged an older desired
// that has since bumped) or, impossibly, ahead. Both are REJECTED by the
// exact-match guard: a converge of stale desired must never mark the current
// desired as done or drive its status. The agent re-polls (the row is still
// outstanding) and converges the current generation.
var ErrStaleGeneration = errors.New("reconcile: generation mismatch")

// ErrUnknownCell covers both "no such cell" and "not your cell" — the caller
// renders 404 for each, so a reconciler token cannot enumerate cells.
var ErrUnknownCell = errors.New("reconcile: unknown cell")

// DesiredService is one row of the agent's poll.
type DesiredService struct {
	ID                 string          `json:"id"`
	CellID             string          `json:"cell_id"`
	EnvID              string          `json:"env_id"`
	Name               string          `json:"name"`
	Product            string          `json:"product"`
	Intent             string          `json:"intent,omitempty"`
	Status             string          `json:"status"`
	Generation         int64           `json:"generation"`
	ObservedGeneration int64           `json:"observed_generation"`
	Desired            json.RawMessage `json:"desired"`
	Shape              json.RawMessage `json:"shape,omitempty"`
	Scaling            json.RawMessage `json:"scaling,omitempty"`
}

const maxLimit = 500

// Desired returns every service in the cell whose generation exceeds
// sinceGeneration. Level-triggered by design (§2 step 2): the FULL desired
// document goes back every time, so the agent renders from it and never diffs
// by memory. A dropped poll therefore costs nothing but latency.
func (s *Service) Desired(ctx context.Context, cell string, sinceGeneration int64, limit int32) ([]DesiredService, error) {
	if _, err := s.q.GetCell(ctx, cell); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnknownCell
		}
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if sinceGeneration < 0 {
		sinceGeneration = 0
	}
	rows, err := s.q.ListDesiredForCell(ctx, store.ListDesiredForCellParams{
		CellID: cell, Generation: sinceGeneration, Limit: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]DesiredService, 0, len(rows))
	for _, r := range rows {
		d := DesiredService{
			ID: r.ID, CellID: r.CellID, EnvID: r.EnvID, Name: r.Name,
			Product: r.Product, Status: r.Status,
			Generation: r.Generation, ObservedGeneration: r.ObservedGeneration,
			Desired: json.RawMessage(r.Desired), Shape: json.RawMessage(r.Shape),
		}
		if r.Intent.Valid {
			d.Intent = r.Intent.String
		}
		if len(r.Scaling) > 0 {
			d.Scaling = json.RawMessage(r.Scaling)
		}
		out = append(out, d)
	}
	return out, nil
}

// Report is one status writeback from the agent.
type Report struct {
	ServiceID          string
	ObservedGeneration int64
	Status             string
	Conditions         json.RawMessage
	Event              string
}

// Writeback records observation and, when the reported status differs from
// what the control plane holds, drives the status machine.
//
// Order matters and is deliberate:
//  1. heartbeat first — an agent that is alive but reporting something invalid
//     must still count as alive, or a bad report would look like a dead cell.
//  2. MarkObserved, whose exact-match guard REJECTS any report not on the
//     current generation (behind or ahead) — see ErrStaleGeneration.
//  3. the status edge, through the existing Transition (events + metering).
//
// Repeating an identical writeback is a no-op: MarkObserved re-sets observed to
// the same value, and an already-current status skips the transition.
// Concurrency is handled by Transition's own FROM-guard, so two agents
// reporting the same edge apply once.
func (s *Service) Writeback(ctx context.Context, cell string, rep Report) (store.Service, error) {
	if _, err := s.q.GetCell(ctx, cell); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Service{}, ErrUnknownCell
		}
		return store.Service{}, err
	}
	if err := s.q.TouchCellHeartbeat(ctx, cell); err != nil {
		return store.Service{}, err
	}

	svc, err := s.q.GetService(ctx, rep.ServiceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Service{}, ErrUnknownCell // no cross-cell probing
		}
		return store.Service{}, err
	}
	// A service in another cell is invisible to this token, same as an unknown
	// one — 404, never 403 (org-fencing convention).
	if svc.CellID != cell {
		return store.Service{}, ErrUnknownCell
	}

	updated, err := s.q.MarkObserved(ctx, store.MarkObservedParams{
		ID: rep.ServiceID, ObservedGeneration: rep.ObservedGeneration,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Service{}, fmt.Errorf("%w: reported %d, desired holds a different generation",
				ErrStaleGeneration, rep.ObservedGeneration)
		}
		return store.Service{}, err
	}

	if rep.Status == "" || rep.Status == updated.Status {
		return updated, nil // observation only — nothing to transition
	}
	orgID, err := s.q.OrgForService(ctx, rep.ServiceID)
	if err != nil {
		return store.Service{}, err
	}
	// via=system: this edge came from the cell converging, not a person.
	return s.trans.Transition(ctx, updated, rep.Status, "system", "system", orgID)
}
