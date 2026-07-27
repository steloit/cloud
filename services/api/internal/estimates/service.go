package estimates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// TTL is how long an estimate stays acceptable. Engineering default (the
// canon W2 example expires within the hour); revisit with pricing churn.
const TTL = time.Hour

type problemError struct{ p problem.Problem }

func (e problemError) Error() string            { return "estimates: " + e.p.Title }
func (e problemError) Problem() problem.Problem { return e.p }

// Service prices and persists estimates.
type Service struct {
	q *store.Queries
}

func NewService(q *store.Queries) *Service { return &Service{q: q} }

// Created is a priced, persisted estimate.
type Created struct {
	Row   store.Estimate
	Lines []Line
}

// Create prices the shapes and persists the estimate. envID/orgID may be
// empty (a pre-project pricing preview) — such an estimate can never be
// accepted, only read.
func (s *Service) Create(ctx context.Context, orgID, envID string, shapes []ShapeInput) (Created, error) {
	if len(shapes) == 0 {
		return Created{}, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "services", Detail: "at least one service shape is required"}})}
	}
	lines, total, err := PriceAll(shapes)
	if err != nil {
		var se ShapeError
		if errors.As(err, &se) {
			return Created{}, problemError{p: problem.ValidationFailed(
				[]problem.FieldError{{Field: se.Field, Detail: se.Detail}})}
		}
		return Created{}, err
	}
	shapesJSON, err := json.Marshal(shapes)
	if err != nil {
		return Created{}, fmt.Errorf("estimates: marshal shapes: %w", err)
	}
	linesJSON, err := json.Marshal(lines)
	if err != nil {
		return Created{}, fmt.Errorf("estimates: marshal lines: %w", err)
	}
	row, err := s.q.InsertEstimate(ctx, store.InsertEstimateParams{
		ID:         ids.New("est"),
		OrgID:      textOrNull(orgID),
		EnvID:      textOrNull(envID),
		Services:   shapesJSON,
		Lines:      linesJSON,
		TotalCents: total,
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(TTL), Valid: true},
	})
	if err != nil {
		return Created{}, fmt.Errorf("estimates: insert: %w", err)
	}
	return Created{Row: row, Lines: lines}, nil
}

// Accept flips a live, env-matching, unaccepted estimate — the provision
// gate's server half (consumed by createService, T3.3). Returns the shapes
// the estimate priced so the caller can verify what it provisions is what
// was accepted.
func (s *Service) Accept(ctx context.Context, estimateID, envID string) (store.Estimate, []ShapeInput, error) {
	row, err := s.q.AcceptEstimate(ctx, store.AcceptEstimateParams{ID: estimateID, EnvID: textOrNull(envID)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Estimate{}, nil, problemError{p: problem.Conflict(
				[]string{"estimate is missing, expired, already used, or priced for another environment"},
				"Create a fresh estimate for this environment and accept it — nothing provisions without one.")}
		}
		return store.Estimate{}, nil, err
	}
	var shapes []ShapeInput
	if err := json.Unmarshal(row.Services, &shapes); err != nil {
		return store.Estimate{}, nil, fmt.Errorf("estimates: stored shapes: %w", err)
	}
	return row, shapes, nil
}

// Shapes returns what an estimate priced, without touching acceptance —
// callers pre-check coverage before burning the one-shot Accept.

// PricedShapes returns the stored shapes AND the lines priced alongside them,
// index-aligned (PriceAll preserves order). The gate needs the line to compare
// against the price the customer was actually SHOWN — recomputing from the
// current table can only ever agree with itself.
func (s *Service) PricedShapes(ctx context.Context, estimateID string) ([]ShapeInput, []Line, error) {
	row, err := s.q.GetEstimate(ctx, estimateID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, problemError{p: problem.Conflict(
				[]string{"estimate not found"},
				"Create a fresh estimate for this environment and accept it — nothing provisions without one.")}
		}
		return nil, nil, err
	}
	var shapes []ShapeInput
	if err := json.Unmarshal(row.Services, &shapes); err != nil {
		return nil, nil, fmt.Errorf("estimates: stored shapes: %w", err)
	}
	var lines []Line
	if len(row.Lines) > 0 {
		if err := json.Unmarshal(row.Lines, &lines); err != nil {
			return nil, nil, fmt.Errorf("estimates: stored lines: %w", err)
		}
	}
	return shapes, lines, nil
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
