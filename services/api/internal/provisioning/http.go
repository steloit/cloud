package provisioning

// T3.2 strict handlers. The composition root embeds this beside
// identity.Handlers in one struct satisfying gen.StrictServerInterface;
// identity's middleware resolves principals, identity's error seam writes
// problem.Carrier errors.

import (
	"context"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

type Handlers struct {
	svc   *Service
	authz *identity.Authorizer
	q     *store.Queries
	idsvc *identity.Service
}

func NewHandlers(svc *Service, authz *identity.Authorizer, q *store.Queries, idsvc *identity.Service) *Handlers {
	return &Handlers{svc: svc, authz: authz, q: q, idsvc: idsvc}
}

// memberOrg 404s non-members (no id probing) and returns the acting user.
func (h *Handlers) memberOrg(ctx context.Context, orgID string) (session.Principal, error) {
	p, ok := session.PrincipalFrom(ctx)
	if !ok {
		return session.Principal{}, identity.ErrNoSession
	}
	if _, err := h.q.GetMemberRole(ctx, store.GetMemberRoleParams{OrgID: orgID, UserID: p.UserID}); err != nil {
		return session.Principal{}, notFound("organization")
	}
	return p, nil
}

// requireOrg authorizes a permission in the org's scope (explained 403s).
func (h *Handlers) requireOrg(ctx context.Context, orgID string, perm rbac.Permission, mutating bool) (string, error) {
	p, ok := session.PrincipalFrom(ctx)
	if !ok {
		return "", identity.ErrNoSession
	}
	if mutating && p.Kind == "token" && p.Scope != "full" {
		return "", identity.ErrScopeDenied
	}
	if err := h.authz.Require(ctx, p, perm, rbac.Scope{OrgID: orgID}); err != nil {
		return "", err
	}
	return p.UserID, nil
}

// ---- mapping ---------------------------------------------------------------

func projectToAPI(p store.Project, envCount int) gen.Project {
	out := gen.Project{Id: p.ID, OrgId: p.OrgID, Name: p.Name}
	health := "ok"
	out.Health = (*gen.ProjectHealth)(&health)
	cost := 0 // services + estimates land with T3.1/T3.3; zero is the true number today
	out.MonthlyCostCents = &cost
	out.EnvCount = &envCount
	created := p.CreatedAt.Time
	out.CreatedAt = &created
	return out
}

func envToAPI(e store.Environment, orgHomeRegion string) gen.Environment {
	region := orgHomeRegion
	if e.RegionOverride.Valid {
		region = e.RegionOverride.String
	}
	out := gen.Environment{Id: e.ID, ProjectId: e.ProjectID, Name: e.Name, Region: region}
	kind := gen.EnvironmentKind(e.Kind)
	out.Kind = &kind
	cost := 0
	out.MonthlyCostCents = &cost
	if e.ExpiresAt.Valid {
		t := e.ExpiresAt.Time
		out.ExpiresAt = &t
	}
	return out
}

// ---- operations ------------------------------------------------------------

func (h *Handlers) CreateProject(ctx context.Context, req gen.CreateProjectRequestObject) (gen.CreateProjectResponseObject, error) {
	actor, err := h.requireOrg(ctx, req.OrgPathParam, "project.create", true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "body", Detail: "required"}})}
	}
	if req.Body.TemplateId != nil {
		return nil, notFound("template") // templates arrive with E9
	}
	org, err := h.idsvc.GetOrg(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	region := ""
	if req.Body.Region != nil {
		region = *req.Body.Region
	}
	prj, _, err := h.svc.CreateProject(ctx, org, req.Body.Name, region, actor)
	if err != nil {
		return nil, err
	}
	return gen.CreateProject201JSONResponse(projectToAPI(prj, 1)), nil
}

func (h *Handlers) ListProjects(ctx context.Context, req gen.ListProjectsRequestObject) (gen.ListProjectsResponseObject, error) {
	if _, err := h.memberOrg(ctx, req.OrgPathParam); err != nil {
		return nil, err
	}
	rows, err := h.svc.ListProjects(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	data := make([]gen.Project, 0, len(rows))
	for _, r := range rows {
		data = append(data, projectToAPI(store.Project{
			ID: r.ID, OrgID: r.OrgID, Name: r.Name, CellID: r.CellID,
			DeletionScheduledAt: r.DeletionScheduledAt, CreatedAt: r.CreatedAt,
		}, int(r.EnvCount)))
	}
	return gen.ListProjects200JSONResponse(gen.ProjectList{Data: &data}), nil
}

// projectScoped resolves the project, 404s non-members, returns both.
func (h *Handlers) projectScoped(ctx context.Context, projectID string) (store.Project, session.Principal, error) {
	prj, err := h.svc.ProjectOrg(ctx, projectID)
	if err != nil {
		return store.Project{}, session.Principal{}, err
	}
	p, err := h.memberOrg(ctx, prj.OrgID)
	if err != nil {
		// a real project the caller can't see is a missing project, not a hint
		return store.Project{}, session.Principal{}, notFound("project")
	}
	return prj, p, nil
}

func (h *Handlers) GetProject(ctx context.Context, req gen.GetProjectRequestObject) (gen.GetProjectResponseObject, error) {
	prj, _, err := h.projectScoped(ctx, req.ProjectPathParam)
	if err != nil {
		return nil, err
	}
	n, err := h.q.CountEnvironments(ctx, prj.ID)
	if err != nil {
		return nil, err
	}
	return gen.GetProject200JSONResponse(projectToAPI(prj, int(n))), nil
}

func (h *Handlers) UpdateProject(ctx context.Context, req gen.UpdateProjectRequestObject) (gen.UpdateProjectResponseObject, error) {
	prj, _, err := h.projectScoped(ctx, req.ProjectPathParam)
	if err != nil {
		return nil, err
	}
	actor, err := h.requireOrg(ctx, prj.OrgID, "project.create", true) // rename rides create rights (G1)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || req.Body.Name == nil {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "name", Detail: "required"}})}
	}
	out, err := h.svc.RenameProject(ctx, prj, *req.Body.Name, actor)
	if err != nil {
		return nil, err
	}
	n, err := h.q.CountEnvironments(ctx, out.ID)
	if err != nil {
		return nil, err
	}
	return gen.UpdateProject200JSONResponse(projectToAPI(out, int(n))), nil
}

func (h *Handlers) DeleteProject(ctx context.Context, req gen.DeleteProjectRequestObject) (gen.DeleteProjectResponseObject, error) {
	prj, _, err := h.projectScoped(ctx, req.ProjectPathParam)
	if err != nil {
		return nil, err
	}
	actor, err := h.requireOrg(ctx, prj.OrgID, "project.delete", true)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteProject(ctx, prj, actor); err != nil {
		return nil, err
	}
	return gen.DeleteProject202Response{}, nil
}

func (h *Handlers) CreateEnvironment(ctx context.Context, req gen.CreateEnvironmentRequestObject) (gen.CreateEnvironmentResponseObject, error) {
	prj, _, err := h.projectScoped(ctx, req.ProjectPathParam)
	if err != nil {
		return nil, err
	}
	actor, err := h.requireOrg(ctx, prj.OrgID, "env.manage", true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "body", Detail: "required"}})}
	}
	region := ""
	if req.Body.Region != nil {
		region = *req.Body.Region
	}
	clone := req.Body.CloneShapeFrom != nil && *req.Body.CloneShapeFrom != ""
	branch := req.Body.Data != nil && *req.Body.Data == "branch"
	env, err := h.svc.CreateEnvironment(ctx, prj, req.Body.Name, region, clone, branch, actor)
	if err != nil {
		return nil, err
	}
	org, err := h.idsvc.GetOrg(ctx, prj.OrgID)
	if err != nil {
		return nil, err
	}
	return gen.CreateEnvironment201JSONResponse(envToAPI(env, org.HomeRegion)), nil
}

func (h *Handlers) ListEnvironments(ctx context.Context, req gen.ListEnvironmentsRequestObject) (gen.ListEnvironmentsResponseObject, error) {
	prj, _, err := h.projectScoped(ctx, req.ProjectPathParam)
	if err != nil {
		return nil, err
	}
	org, err := h.idsvc.GetOrg(ctx, prj.OrgID)
	if err != nil {
		return nil, err
	}
	rows, err := h.svc.ListEnvironments(ctx, prj.ID)
	if err != nil {
		return nil, err
	}
	data := make([]gen.Environment, 0, len(rows))
	for _, e := range rows {
		data = append(data, envToAPI(e, org.HomeRegion))
	}
	return gen.ListEnvironments200JSONResponse(gen.EnvironmentList{Data: &data}), nil
}
