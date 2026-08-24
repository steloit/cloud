package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/provisioning"
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

	// ObservedStatus maps a cell's report onto a status legal from the service's
	// CURRENT one. It is on this interface for the same reason Transition is:
	// the machine lives in provisioning, and reconcile decides only WHEN to
	// consult it.
	ObservedStatus(from, observed string) provisioning.Observation
}

// The status machine's Observation type lives in `provisioning`, next to the
// machine itself, and is named in the Transitioner interface above.
//
// Importing that VALUE type is not the coupling the interface exists to avoid:
// the interface is here so this package never reimplements the legal edges, the
// spine events or the metering rule, and so it can be faked in tests. Go has no
// covariant returns, so a mirrored struct here cannot satisfy a method that
// returns the real one — and making `provisioning` import `reconcile` instead
// would point the domain at its own caller.

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

// ErrNotConverged is a report that did NOT finish the generation: the status
// machine came to rest on a status that is still transient (`provisioning`,
// `degraded`), or it could not place the report at all. Either way the row
// deliberately stays outstanding, so the next tick re-observes it. Rendered as
// a 409 — a normal, expected event, not a failure.
var ErrNotConverged = errors.New("reconcile: report does not finish this generation")

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
	// The poll is the liveness signal for a QUIESCENT cell: a fully-converged
	// cell has no outstanding work, so it never writes back, so the status-call
	// heartbeat would freeze and a health check (O4) would call a healthy cell
	// dead. The agent polls every tick regardless of work, so touch the
	// heartbeat here — the cell is "seen" whenever it asks for desired state.
	if err := s.q.TouchCellHeartbeat(ctx, cell); err != nil {
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
//  2. generation pre-check — a report not on the current generation is rejected
//     before anything mutates (the AC's behind-report scenario).
//  3. the status edge FIRST (events + metering, via the existing Transition),
//  4. then MarkObserved — observed_generation advances ONLY after a durable
//     edge, so a failed transition never strands the row out of the outstanding
//     set. MarkObserved's exact-match guard is the atomic backstop for the
//     read-then-check race.
//
// Repeating an identical writeback is a no-op ONCE converged (an unsettled
// report is deliberately repeatable — that is how the second hop lands): an already-current status skips
// the transition, and MarkObserved re-sets observed to the same value.
// Concurrency is handled by Transition's own FROM-guard, so two agents
// reporting the same edge apply the edge once.
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

	// Generation pre-check BEFORE any transition: a report for a generation other
	// than the one desired holds now (behind or ahead) drives nothing — the AC's
	// behind-report scenario must leave the row untouched. This read-then-check
	// has a microsecond TOCTOU with a concurrent desired bump; MarkObserved's SQL
	// guard (generation = $2) is the atomic backstop below.
	if svc.Generation != rep.ObservedGeneration {
		return store.Service{}, fmt.Errorf("%w: reported %d, desired holds %d",
			ErrStaleGeneration, rep.ObservedGeneration, svc.Generation)
	}

	// Order is deliberate and load-bearing: the status edge runs FIRST, and
	// observed_generation advances ONLY after it durably lands. If Transition
	// fails — a concurrent status change, or a mid-request DB error; the
	// illegal-edge case is no longer reachable from here, because ObservedStatus
	// only ever emits edges CanTransition accepts — observed has NOT advanced,
	// so the row stays outstanding and the next tick retries it — the whole
	// retry story depends on this. The reverse order stranded the row: observed
	// advanced, the row left the outstanding set, and a failed edge was lost.
	//
	// THE REPORT IS MAPPED, NOT USED RAW. The agent reads only the CNPG phase, so
	// it answers identically whatever state the row is in — while this edge asks
	// "is that legal from svc.Status". ADR-024 has no `ready → failed`, so a
	// cluster that breaks while READY made the agent report `failed`, Transition
	// rejected it every tick, observed_generation never advanced, and the service
	// was retried forever with nothing visible to the customer. US-3.3h.
	obs := s.trans.ObservedStatus(svc.Status, rep.Status)
	if to, ok := obs.Edge(); ok {
		orgID, err := s.q.OrgForService(ctx, rep.ServiceID)
		if err != nil {
			return store.Service{}, err
		}
		// via=system: this edge came from the cell converging, not a person.
		if _, err := s.trans.Transition(ctx, svc, to, "system", "system", orgID); err != nil {
			return store.Service{}, err // observed NOT advanced — row stays outstanding
		}
	}
	// An UNSETTLED hop must not advance observation. The machine converges only
	// onto a settled status the cell actually reported, so this covers both "the
	// edge was taken and needs one more tick" (`failed` + healthy → provisioning)
	// and "the report could not be placed" (`provisioning` + degraded). Marking
	// either converged strands the row, because ListDesiredForCell selects on
	// observed_generation < generation.
	//
	// The error names BOTH sides: it is the only trace an unplaceable report
	// leaves, it reaches the operator in the 409 body, and the agent logs it.
	if !obs.Converged() {
		return store.Service{}, fmt.Errorf("%w: %s reported %q from %q",
			ErrNotConverged, rep.ServiceID, rep.Status, svc.Status)
	}

	// The transition (if any) is durable; now record that the cell converged this
	// generation. The exact-match guard is the atomic backstop for the TOCTOU
	// above: if desired bumped concurrently, this rejects and the row stays
	// outstanding (the transition that ran was for the now-superseded generation,
	// which the next tick reconciles). Idempotent replays re-set the same value.
	updated, err := s.q.MarkObserved(ctx, store.MarkObservedParams{
		ID: rep.ServiceID, ObservedGeneration: rep.ObservedGeneration,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Service{}, fmt.Errorf("%w: reported %d, desired moved concurrently",
				ErrStaleGeneration, rep.ObservedGeneration)
		}
		return store.Service{}, err
	}
	return updated, nil
}
