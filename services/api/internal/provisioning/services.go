package provisioning

// T3.3: service rows + the guarded status machine (ADR-024 vocabulary:
// provisioning|ready|degraded|failed|suspended|deleting — `ready`, never
// `running`; metering starts at ready). Rows are DESIRED STATE (D9): the
// reconciler (T3.4+, cell-agent) converges actual state; nothing here talks
// to infrastructure.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/estimates"
	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// transitions is the closed status machine. deleting is terminal (the
// reconciler removes the row after teardown + final backup).
var transitions = map[string][]string{
	"provisioning": {"ready", "failed", "deleting"}, // deleting = cancel-the-create
	"ready":        {"degraded", "suspended", "deleting"},
	"degraded":     {"ready", "failed", "deleting"},
	"failed":       {"provisioning", "deleting"}, // retry re-provisions
	"suspended":    {"ready", "deleting"},
	"deleting":     {},
}

// CanTransition reports whether from → to is a legal edge.
func CanTransition(from, to string) bool {
	for _, t := range transitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

// provisioningSteps is the C4 timeline, born with the row.
func provisioningSteps() []byte {
	steps := []map[string]string{
		{"step": "allocate", "status": "active"},
		{"step": "configure", "status": "pending"},
		{"step": "network/credentials", "status": "pending"},
		{"step": "first backup", "status": "pending"},
		{"step": "ready", "status": "pending"},
	}
	b, _ := json.Marshal(steps)
	return b
}

// CreateServiceInput is the decoded ServiceCreate.
type CreateServiceInput struct {
	Name       string
	Product    string
	Intent     string
	Shape      map[string]any
	EstimateID string
	ActorID    string
}

// CreateService enforces the estimate-before-provision law AT THE API LAYER:
// the estimate must accept (one-shot, env-fenced, live) AND contain a shape
// matching this service — what provisions is what was priced.
func (s *Service) CreateService(ctx context.Context, est *estimates.Service, env store.Environment, orgID string, in CreateServiceInput) (store.Service, error) {
	if in.Name == "" {
		return store.Service{}, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "name", Detail: "required"}})}
	}
	if in.EstimateID == "" {
		return store.Service{}, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "estimate_id", Detail: "required — nothing provisions without an accepted estimate (F2)"}})}
	}
	// Price line for THIS shape (also validates product/size before burning
	// the one-shot estimate).
	line, err := estimates.Price(estimates.ShapeInput{
		Product: in.Product, Intent: in.Intent, Name: in.Name, Shape: in.Shape,
	})
	if err != nil {
		var se estimates.ShapeError
		if errors.As(err, &se) {
			return store.Service{}, problemError{p: problem.ValidationFailed(
				[]problem.FieldError{{Field: se.Field, Detail: se.Detail}})}
		}
		return store.Service{}, err
	}

	// Coverage pre-check BEFORE burning the one-shot estimate (estimate rows
	// are immutable, so this is race-free): a mistyped create keeps the
	// estimate usable — better DX, same law.
	priced, err := est.Shapes(ctx, in.EstimateID)
	if err != nil {
		return store.Service{}, err
	}
	matched := false
	for _, sh := range priced {
		if sh.Product == in.Product && priceOf(sh) == line.MonthlyCents {
			matched = true
			break
		}
	}
	if !matched {
		return store.Service{}, problemError{p: problem.Conflict(
			[]string{"the estimate does not cover this shape"},
			"Estimate the exact shape you are creating, accept it, then create — the estimate IS the contract.")}
	}
	if _, _, err := est.Accept(ctx, in.EstimateID, env.ID); err != nil {
		return store.Service{}, err
	}

	shapeJSON, err := json.Marshal(in.Shape)
	if err != nil {
		return store.Service{}, fmt.Errorf("provisioning: marshal shape: %w", err)
	}
	row, err := s.q.InsertService(ctx, store.InsertServiceParams{
		ID: ids.New("svc"), EnvID: env.ID, Name: in.Name, Product: in.Product,
		Intent:               pgtype.Text{String: line.Intent, Valid: true},
		Shape:                shapeJSON,
		ProvisioningSteps:    provisioningSteps(),
		MonthlyEstimateCents: line.MonthlyCents,
		EstimateID:           pgtype.Text{String: in.EstimateID, Valid: true},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"a service with this name already exists in the environment"},
				"Pick a different name.")}
		}
		return store.Service{}, err
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "lifecycle", Via: "user", Actor: in.ActorID,
		Action: "service.created", Subject: row.ID,
		Detail: []byte(`{"name":` + strconv.Quote(in.Name) + `,"product":` + strconv.Quote(in.Product) + `,"estimate":` + strconv.Quote(in.EstimateID) + `}`),
	})
	return row, nil
}

// priceOf prices a stored estimate shape (already validated at estimate time).
func priceOf(sh estimates.ShapeInput) int64 {
	l, err := estimates.Price(sh)
	if err != nil {
		return -1
	}
	return l.MonthlyCents
}

// Transition moves a service along a legal edge, atomically (the SQL guard
// re-checks FROM), records the lifecycle event, and marks step timelines on
// the terminal provisioning edges. Illegal edges are conflicts, not crashes.
func (s *Service) Transition(ctx context.Context, svc store.Service, to, via, actor string, orgID string) (store.Service, error) {
	if !CanTransition(svc.Status, to) {
		return store.Service{}, problemError{p: problem.Conflict(
			[]string{"illegal status transition " + svc.Status + " → " + to},
			"Legal next states from "+svc.Status+": "+fmt.Sprint(transitions[svc.Status])+" (ADR-024).")}
	}
	var steps []byte
	if svc.Status == "provisioning" && to == "ready" {
		b, _ := json.Marshal([]map[string]string{
			{"step": "allocate", "status": "done"},
			{"step": "configure", "status": "done"},
			{"step": "network/credentials", "status": "done"},
			{"step": "first backup", "status": "done"},
			{"step": "ready", "status": "done"},
		})
		steps = b
	}
	row, err := s.q.SetServiceStatus(ctx, store.SetServiceStatusParams{
		ID: svc.ID, Status: svc.Status, Status_2: to, Steps: steps,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"service state changed concurrently"},
				"Re-read the service and retry the transition.")}
		}
		return store.Service{}, err
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "lifecycle", Via: via, Actor: actor,
		Action: "service." + to, Subject: svc.ID,
		Detail: []byte(`{"from":` + strconv.Quote(svc.Status) + `}`),
	})
	return row, nil
}

// ServiceOrg resolves service → org (404 for unknown ids — no probing).
func (s *Service) ServiceOrg(ctx context.Context, serviceID string) (store.Service, string, error) {
	svc, err := s.q.GetService(ctx, serviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Service{}, "", notFound("service")
	}
	if err != nil {
		return store.Service{}, "", err
	}
	orgID, err := s.q.OrgForService(ctx, serviceID)
	if err != nil {
		return store.Service{}, "", err
	}
	return svc, orgID, nil
}

// UpdateService — shape/scaling are desired-state edits (repriced); the
// manual override requires a reason and auto-expires in 24h (D22).
func (s *Service) UpdateService(ctx context.Context, svc store.Service, orgID, actorID string, shape map[string]any, scaling, override []byte) (store.Service, error) {
	params := store.UpdateServiceShapeParams{ID: svc.ID}
	if shape != nil {
		// merge over the existing shape: PATCH semantics, absent keys survive
		var current map[string]any
		_ = json.Unmarshal(svc.Shape, &current)
		if current == nil {
			current = map[string]any{}
		}
		for k, v := range shape {
			current[k] = v
		}
		merged, err := json.Marshal(current)
		if err != nil {
			return store.Service{}, err
		}
		line, err := estimates.Price(estimates.ShapeInput{Product: svc.Product, Name: svc.Name, Shape: current})
		if err != nil {
			var se estimates.ShapeError
			if errors.As(err, &se) {
				return store.Service{}, problemError{p: problem.ValidationFailed(
					[]problem.FieldError{{Field: se.Field, Detail: se.Detail}})}
			}
			return store.Service{}, err
		}
		params.Shape = merged
		params.MonthlyEstimateCents = pgtype.Int8{Int64: line.MonthlyCents, Valid: true}
	}
	params.Scaling = scaling
	params.Override = override
	row, err := s.q.UpdateServiceShape(ctx, params)
	if err != nil {
		return store.Service{}, err
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "scale", Via: "user", Actor: actorID,
		Action: "service.updated", Subject: svc.ID,
	})
	return row, nil
}

// DeleteService — desired state → deleting (202). The final backup + actual
// teardown are the driver's job (T3.4/US-3.5); dependents (bindings, T3.6)
// join the 409 check when they exist.
func (s *Service) DeleteService(ctx context.Context, svc store.Service, orgID, actorID string) error {
	if svc.Status == "deleting" {
		return problemError{p: problem.Conflict([]string{"deletion already in progress"},
			"The service is already deleting; the final backup will be recorded.")}
	}
	// U6: dependents that will knowingly break are NAMED, all of them.
	deps, err := s.q.ActiveBindingsToTarget(ctx, pgtype.Text{String: svc.ID, Valid: true})
	if err != nil {
		return err
	}
	if len(deps) > 0 {
		reasons := make([]string, 0, len(deps))
		for _, d := range deps {
			reasons = append(reasons, "service "+d.SourceName+" binds to this service ("+d.ID+")")
		}
		return problemError{p: problem.Conflict(reasons,
			"Unbind the listed services first — deleting would knowingly break them (U6).")}
	}
	_, err = s.Transition(ctx, svc, "deleting", "user", actorID, orgID)
	return err
}

func (s *Service) ListServices(ctx context.Context, envID string) ([]store.Service, error) {
	return s.q.ListServicesForEnv(ctx, envID)
}
