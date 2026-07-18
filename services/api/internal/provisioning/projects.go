// Package provisioning is the M4 module. T3.2 lays the containment rows —
// projects and environments — behind the contract. NOTHING provisions here
// (D9: desired state only; the reconciler converges it when cells exist);
// cell_id rides every row (invariant 1) and never surfaces (D8).
package provisioning

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
	"github.com/steloit/cloud/services/api/internal/secrets"
)

// problemError carries a catalog problem through the one strict-server error
// seam (problem.Carrier).
type problemError struct{ p problem.Problem }

func (e problemError) Error() string            { return "provisioning: " + e.p.Title }
func (e problemError) Problem() problem.Problem { return e.p }

// NotFound is exported so handlers can 404 without probing semantics leaks.
func notFound(what string) error { return problemError{p: problem.NotFound(what)} }

// projectAllowance is the B5 plan matrix row "Projects": Free 1 / Pro 3 /
// Business+ unlimited. Data, one place.
var projectAllowance = map[string]int{"free": 1, "pro": 3, "business": -1, "enterprise": -1}

// nextPlanFor names the upgrade that lifts the project limit (F2 "gate with
// reason").
func nextPlanFor(plan string) string {
	if plan == "free" {
		return "pro"
	}
	return "business"
}

type Service struct {
	db    *pgxpool.Pool
	q     *store.Queries
	rec   *events.Recorder
	vault *secrets.Vault // consumed by bindings (T3.6); credentials never at rest in plaintext
}

func NewService(db *pgxpool.Pool, rec *events.Recorder, vault *secrets.Vault) *Service {
	return &Service{db: db, q: store.New(db), rec: rec, vault: vault}
}

func (s *Service) record(ctx context.Context, in events.Input) {
	if s.rec == nil {
		return
	}
	_, _ = s.rec.Append(ctx, in)
}

// OrgForEnv implements events.EnvResolver — the T2.5 seam closes here.
func (s *Service) OrgForEnv(ctx context.Context, envID string) (string, error) {
	orgID, err := s.q.OrgForEnvironment(ctx, envID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", events.ErrEnvNotFound
	}
	return orgID, err
}

// CreateProject creates the project AND its implicit production environment
// in one transaction (ADR-037: born, not created). The plan gate runs first:
// at the allowance, 402 names the plan that lifts it.
func (s *Service) CreateProject(ctx context.Context, org store.Org, name, region, actorID string) (store.Project, store.Environment, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return store.Project{}, store.Environment{}, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "name", Detail: "required"}})}
	}
	if org.DeletionScheduledAt.Valid {
		return store.Project{}, store.Environment{}, problemError{p: problem.Conflict(
			[]string{"organization is scheduled for deletion"},
			"Creation is blocked while deletion is scheduled.")}
	}
	if allowance := projectAllowance[org.Plan]; allowance >= 0 {
		n, err := s.q.CountProjects(ctx, org.ID)
		if err != nil {
			return store.Project{}, store.Environment{}, err
		}
		if int(n) >= allowance {
			return store.Project{}, store.Environment{}, problemError{p: problem.PlanGated(nextPlanFor(org.Plan))}
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return store.Project{}, store.Environment{}, fmt.Errorf("provisioning: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	prj, err := q.CreateProject(ctx, store.CreateProjectParams{ID: ids.New("prj"), OrgID: org.ID, Name: name})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return store.Project{}, store.Environment{}, problemError{p: problem.Conflict(
				[]string{"a project with this name already exists in the organization"},
				"Pick a different name.")}
		}
		return store.Project{}, store.Environment{}, fmt.Errorf("provisioning: create project: %w", err)
	}
	// The implicit environment: region_override only when the caller chose a
	// region different from the org default — otherwise it inherits.
	var override pgtype.Text
	if region != "" && region != org.HomeRegion {
		override = pgtype.Text{String: region, Valid: true}
	}
	env, err := q.CreateEnvironment(ctx, store.CreateEnvironmentParams{
		ID: ids.New("env"), ProjectID: prj.ID, Name: "production", RegionOverride: override, Kind: "standard",
	})
	if err != nil {
		return store.Project{}, store.Environment{}, fmt.Errorf("provisioning: implicit env: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Project{}, store.Environment{}, fmt.Errorf("provisioning: commit: %w", err)
	}
	s.record(ctx, events.Input{
		OrgID: org.ID, Kind: "lifecycle", Via: "user", Actor: actorID,
		Action: "project.created", Subject: prj.ID,
		Detail: []byte(`{"name":` + strconv.Quote(name) + `}`),
	})
	return prj, env, nil
}

// ProjectOrg resolves a project to its org for authorization; missing ids
// are 404 (no probing).
func (s *Service) ProjectOrg(ctx context.Context, projectID string) (store.Project, error) {
	prj, err := s.q.GetProject(ctx, projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Project{}, notFound("project")
	}
	return prj, err
}

func (s *Service) ListProjects(ctx context.Context, orgID string) ([]store.ListProjectsForOrgRow, error) {
	return s.q.ListProjectsForOrg(ctx, orgID)
}

func (s *Service) RenameProject(ctx context.Context, prj store.Project, name, actorID string) (store.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return store.Project{}, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "name", Detail: "required"}})}
	}
	out, err := s.q.RenameProject(ctx, store.RenameProjectParams{ID: prj.ID, Name: name})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return store.Project{}, problemError{p: problem.Conflict(
				[]string{"a project with this name already exists in the organization"},
				"Pick a different name.")}
		}
		return store.Project{}, err
	}
	s.record(ctx, events.Input{
		OrgID: prj.OrgID, Kind: "lifecycle", Via: "user", Actor: actorID,
		Action: "project.updated", Subject: prj.ID,
		Detail: []byte(`{"name":` + strconv.Quote(name) + `}`),
	})
	return out, nil
}

// DeleteProject — 202 schedule. Blocked with ALL reasons while non-implicit
// environments exist (services join the check at T3.3). The implicit
// production env never blocks: it is born, not created (ADR-037).
func (s *Service) DeleteProject(ctx context.Context, prj store.Project, actorID string) error {
	envs, err := s.q.ListEnvironments(ctx, prj.ID)
	if err != nil {
		return err
	}
	var reasons []string
	for _, e := range envs {
		if e.Name != "production" {
			reasons = append(reasons, "environment "+e.Name+" exists")
		}
	}
	if len(reasons) > 0 {
		return problemError{p: problem.Conflict(reasons,
			"Delete the listed environments first, or acknowledge the cascade when that ships (U6).")}
	}
	n, err := s.q.ScheduleProjectDeletion(ctx, prj.ID)
	if err != nil {
		return err
	}
	if n == 0 {
		return problemError{p: problem.Conflict([]string{"deletion already scheduled"},
			"The project is already scheduled for deletion.")}
	}
	s.record(ctx, events.Input{
		OrgID: prj.OrgID, Kind: "lifecycle", Via: "user", Actor: actorID,
		Action: "project.deletion_scheduled", Subject: prj.ID,
	})
	return nil
}

// CreateEnvironment — C8. clone_shape_from / data:branch need the CNPG
// driver (T3.4): refused loudly with remediation, never silently accepted.
func (s *Service) CreateEnvironment(ctx context.Context, prj store.Project, name, region string, clone, branch bool, actorID string) (store.Environment, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return store.Environment{}, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "name", Detail: "required"}})}
	}
	if clone || branch {
		return store.Environment{}, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "clone_shape_from", Detail: "shape cloning and data branches arrive with the database driver (T3.4); create the environment empty for now"}})}
	}
	var override pgtype.Text
	if region != "" {
		override = pgtype.Text{String: region, Valid: true}
	}
	env, err := s.q.CreateEnvironment(ctx, store.CreateEnvironmentParams{
		ID: ids.New("env"), ProjectID: prj.ID, Name: name, RegionOverride: override, Kind: "standard",
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return store.Environment{}, problemError{p: problem.Conflict(
				[]string{"an environment with this name already exists in the project"},
				"Pick a different name.")}
		}
		return store.Environment{}, err
	}
	s.record(ctx, events.Input{
		OrgID: prj.OrgID, Kind: "lifecycle", Via: "user", Actor: actorID,
		Action: "env.created", Subject: env.ID,
		Detail: []byte(`{"name":` + strconv.Quote(name) + `,"project":` + strconv.Quote(prj.ID) + `}`),
	})
	return env, nil
}

func (s *Service) ListEnvironments(ctx context.Context, projectID string) ([]store.Environment, error) {
	return s.q.ListEnvironments(ctx, projectID)
}
