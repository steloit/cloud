package provisioning

// T4.3: deployment records — IMMUTABLE history (DP1) + rollback. Records are
// desired state: the build pipeline (T4.2, P1-gated) drives queued→…→live;
// today records are created, numbered, and marked on the spine (US-4.4:
// every chart of the env can render #N + sha).

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// deployTransitions is the DP1 state machine (the pipeline drives it).
var deployTransitions = map[string][]string{
	"queued":    {"building", "aborted"},
	"building":  {"migrating", "failed", "aborted"},
	"migrating": {"canary", "failed", "aborted"},
	"canary":    {"verifying", "failed", "aborted"},
	"verifying": {"live", "failed", "aborted"},
	"live":      {"rolled_back"},
	// failed / rolled_back / aborted are terminal
}

func CanDeployTransition(from, to string) bool {
	for _, t := range deployTransitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

type CreateDeploymentInput struct {
	ServiceID   string
	GitSha      string
	PromoteFrom string
	ActorID     string
	rollbackOf  string // internal: set by Rollback
}

// CreateDeployment numbers the record per-env (unique index arbitrates
// races; one retry) and lands the deploy marker on the spine.
func (s *Service) CreateDeployment(ctx context.Context, env store.Environment, orgID string, in CreateDeploymentInput) (store.Deployment, error) {
	if in.ServiceID == "" {
		return store.Deployment{}, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "service", Detail: "required"}})}
	}
	svc, err := s.q.GetService(ctx, in.ServiceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Deployment{}, notFound("service")
	}
	if err != nil {
		return store.Deployment{}, err
	}
	if svc.EnvID != env.ID {
		return store.Deployment{}, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "service", Detail: "service is not in this environment"}})}
	}
	if svc.Product != "web" && svc.Product != "worker" {
		return store.Deployment{}, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "service", Detail: "deployments target compute services (web, worker); databases change through their own lifecycle"}})}
	}
	var promotedFrom pgtype.Text
	if in.PromoteFrom != "" {
		src, err := s.q.GetDeployment(ctx, in.PromoteFrom)
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Deployment{}, notFound("promote_from deployment")
		}
		if err != nil {
			return store.Deployment{}, err
		}
		promotedFrom = pgtype.Text{String: src.ID, Valid: true}
		if in.GitSha == "" {
			in.GitSha = src.GitSha // promotion carries the promoted image's sha
		}
	}
	var rollbackOf pgtype.Text
	if in.rollbackOf != "" {
		rollbackOf = pgtype.Text{String: in.rollbackOf, Valid: true}
	}

	var row store.Deployment
	for attempt := 0; attempt < 2; attempt++ {
		number, err := s.q.NextDeploymentNumber(ctx, env.ID)
		if err != nil {
			return store.Deployment{}, err
		}
		row, err = s.q.InsertDeployment(ctx, store.InsertDeploymentParams{
			ID: ids.New("dep"), Number: number, EnvID: env.ID, ServiceID: svc.ID,
			GitSha: in.GitSha, Actor: in.ActorID,
			PromotedFrom: promotedFrom, RollbackOf: rollbackOf,
		})
		if err == nil {
			break
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && attempt == 0 {
			continue // lost the number race once; take the next number
		}
		return store.Deployment{}, fmt.Errorf("provisioning: insert deployment: %w", err)
	}
	// US-4.4: the deploy marker — number + sha, so ANY chart of this env can
	// render it (QA scenario 1's replay depends on this).
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "deploy", Via: "user", Actor: in.ActorID,
		Action: "deploy.created", Subject: row.ID,
		Detail: []byte(`{"number":` + strconv.Itoa(int(row.Number)) + `,"sha":` + strconv.Quote(row.GitSha) + `,"service":` + strconv.Quote(svc.Name) + `}`),
	})
	return row, nil
}

func (s *Service) ListDeployments(ctx context.Context, envID string) ([]store.Deployment, error) {
	return s.q.ListDeploymentsForEnv(ctx, envID)
}

// DeploymentOrg resolves deployment → env → org (404, no probing).
func (s *Service) DeploymentOrg(ctx context.Context, depID string) (store.Deployment, store.Environment, string, error) {
	dep, err := s.q.GetDeployment(ctx, depID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Deployment{}, store.Environment{}, "", notFound("deployment")
	}
	if err != nil {
		return store.Deployment{}, store.Environment{}, "", err
	}
	env, err := s.q.GetEnvironment(ctx, dep.EnvID)
	if err != nil {
		return store.Deployment{}, store.Environment{}, "", err
	}
	orgID, err := s.q.OrgForEnvironment(ctx, env.ID)
	if err != nil {
		return store.Deployment{}, store.Environment{}, "", err
	}
	return dep, env, orgID, nil
}

// Rollback creates a NEW deployment record redeploying the previous image
// (history is append-only — nothing rewrites). The rolled-back deployment is
// marked; migrations never auto-revert (expand-contract).
func (s *Service) Rollback(ctx context.Context, dep store.Deployment, env store.Environment, orgID, actorID string) (store.Deployment, error) {
	prev, err := s.q.PreviousDeployment(ctx, store.PreviousDeploymentParams{
		EnvID: env.ID, ServiceID: dep.ServiceID, Number: dep.Number,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Deployment{}, problemError{p: problem.Conflict(
			[]string{"no earlier successful deployment of this service exists"},
			"Rollback redeploys the previous image; the first deployment has nothing to return to.")}
	}
	if err != nil {
		return store.Deployment{}, err
	}
	row, err := s.CreateDeployment(ctx, env, orgID, CreateDeploymentInput{
		ServiceID: dep.ServiceID, GitSha: prev.GitSha, ActorID: actorID, rollbackOf: dep.ID,
	})
	if err != nil {
		return store.Deployment{}, err
	}
	// mark the superseded deployment if it was live (state machine edge)
	if dep.State == "live" {
		if _, err := s.q.SetDeploymentState(ctx, store.SetDeploymentStateParams{
			ID: dep.ID, State: "live", State_2: "rolled_back",
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return store.Deployment{}, err
		}
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "deploy", Via: "user", Actor: actorID,
		Action: "deploy.rolled_back", Subject: dep.ID,
		Detail: []byte(`{"redeploys":` + strconv.Quote(prev.GitSha) + `,"new":` + strconv.Quote(row.ID) + `}`),
	})
	return row, nil
}
