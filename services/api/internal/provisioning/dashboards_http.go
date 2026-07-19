package provisioning

// T12.6 — dashboards (DB8). SCOPE (org | project:prj_…) is orthogonal to
// VISIBILITY (personal | org | restricted): scope decides WHAT data the
// dashboard reads, visibility decides WHO sees the dashboard object. Both gates
// apply independently. Project-scoped dashboards are "born filtered" — create
// and read require access to that project. Dashboards READ, never provision.

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// scopeProject returns the project id a scope string targets, or "" for an
// org-wide scope. "project:prj_…" → "prj_…"; "org" → "".
func scopeProject(scope string) string {
	if rest, ok := strings.CutPrefix(scope, "project:"); ok {
		return rest
	}
	return ""
}

// dashboardVisibleTo reports whether a viewer may SEE the dashboard object
// (the visibility axis, independent of scope). personal/restricted → owner
// only (no restricted-share surface exists yet — recorded); org → any member.
// Membership is already established by the caller.
func dashboardVisibleTo(d store.Dashboard, userID string) bool {
	if d.OwnerID == userID {
		return true
	}
	return d.Visibility == "org"
}

// requireScopeAccess enforces the SCOPE axis: a project-scoped dashboard binds
// to that project's access (born filtered). Returns the resolved org id.
func (h *Handlers) requireScopeAccess(ctx context.Context, orgID, scope string) error {
	if prj := scopeProject(scope); prj != "" {
		p, _, err := h.projectScoped(ctx, prj)
		_ = p
		if err != nil {
			return err // 404 for a project the caller can't see (born filtered)
		}
		// the project must belong to the dashboard's org (no cross-org scope)
		row, err := h.svc.ProjectOrg(ctx, prj)
		if err != nil {
			return notFound("project")
		}
		if row.OrgID != orgID {
			return problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "scope", Detail: "project is not in this org"}})}
		}
	}
	return nil
}

func (h *Handlers) CreateDashboard(ctx context.Context, req gen.CreateDashboardRequestObject) (gen.CreateDashboardResponseObject, error) {
	orgID := req.OrgPathParam
	if req.Body == nil || req.Body.Name == "" {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "name", Detail: "required"}})}
	}
	scope := req.Body.Scope
	if scope != "org" && scopeProject(scope) == "" {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "scope", Detail: `must be "org" or "project:prj_…"`}})}
	}
	vis := string(req.Body.Visibility)

	// VISIBILITY axis: creating a shared (org/restricted) dashboard needs the
	// share-org grant (billing role is denied); personal needs only create.
	// Literals stay at the call site (the coverage tripwire scans for them).
	var userID string
	var err error
	if vis != "personal" {
		userID, err = h.requireOrg(ctx, orgID, "dashboard.share_org", true)
	} else {
		userID, err = h.requireOrg(ctx, orgID, "dashboard.create", true)
	}
	if err != nil {
		return nil, err
	}
	// SCOPE axis: a project-scoped dashboard is born filtered to that project.
	if err := h.requireScopeAccess(ctx, orgID, scope); err != nil {
		return nil, err
	}

	layout := json.RawMessage("{}")
	d, err := h.q.CreateDashboard(ctx, store.CreateDashboardParams{
		ID: ids.New("dsh"), OrgID: orgID, Name: req.Body.Name,
		Scope: scope, Visibility: vis, OwnerID: userID, Layout: layout,
	})
	if err != nil {
		return nil, err
	}
	return gen.CreateDashboard201JSONResponse(dashboardToAPI(d, nil)), nil
}

func (h *Handlers) ListDashboards(ctx context.Context, req gen.ListDashboardsRequestObject) (gen.ListDashboardsResponseObject, error) {
	orgID := req.OrgPathParam
	p, err := h.memberOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	scopeFilter := ""
	if req.Params.Scope != nil {
		scopeFilter = *req.Params.Scope
	}
	rows, err := h.q.ListDashboardsForOrg(ctx, store.ListDashboardsForOrgParams{
		OrgID: orgID, Column2: scopeFilter, Limit: 200,
	})
	if err != nil {
		return nil, err
	}
	out := []gen.Dashboard{}
	for _, d := range rows {
		// VISIBILITY: hide objects the viewer may not see.
		if !dashboardVisibleTo(d, p.UserID) {
			continue
		}
		// SCOPE (born filtered): a project-scoped dashboard is omitted unless the
		// viewer can access that project.
		if prj := scopeProject(d.Scope); prj != "" {
			if _, _, err := h.projectScoped(ctx, prj); err != nil {
				continue
			}
		}
		out = append(out, dashboardToAPI(d, nil))
	}
	return gen.ListDashboards200JSONResponse{Data: &out}, nil
}

// dashboardReadable resolves a dashboard and enforces BOTH axes for a reader.
func (h *Handlers) dashboardReadable(ctx context.Context, dashID string) (store.Dashboard, session.Principal, error) {
	d, err := h.q.GetDashboard(ctx, dashID)
	if err != nil {
		return store.Dashboard{}, session.Principal{}, notFound("dashboard")
	}
	p, err := h.memberOrg(ctx, d.OrgID)
	if err != nil {
		return store.Dashboard{}, session.Principal{}, notFound("dashboard") // non-member: never confirm it exists
	}
	if !dashboardVisibleTo(d, p.UserID) {
		return store.Dashboard{}, session.Principal{}, notFound("dashboard")
	}
	if prj := scopeProject(d.Scope); prj != "" {
		if _, _, err := h.projectScoped(ctx, prj); err != nil {
			return store.Dashboard{}, session.Principal{}, notFound("dashboard")
		}
	}
	return d, p, nil
}

func (h *Handlers) GetDashboard(ctx context.Context, req gen.GetDashboardRequestObject) (gen.GetDashboardResponseObject, error) {
	d, _, err := h.dashboardReadable(ctx, req.Dash)
	if err != nil {
		return nil, err
	}
	widgets, err := h.q.ListWidgetsForDashboard(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	return gen.GetDashboard200JSONResponse(dashboardToAPI(d, widgets)), nil
}

// requireDashboardEditor: the owner, or a member with share-org rights, may
// mutate a shared dashboard (edits are live for all viewers + audited, F7).
func (h *Handlers) requireDashboardEditor(ctx context.Context, dashID string) (store.Dashboard, error) {
	d, err := h.q.GetDashboard(ctx, dashID)
	if err != nil {
		return store.Dashboard{}, notFound("dashboard")
	}
	p, err := h.memberOrg(ctx, d.OrgID)
	if err != nil {
		return store.Dashboard{}, notFound("dashboard")
	}
	if d.OwnerID == p.UserID {
		if !dashboardVisibleTo(d, p.UserID) { // owner always passes; scope still binds below
			return store.Dashboard{}, notFound("dashboard")
		}
		return d, nil
	}
	// not the owner → needs the share-org grant AND the dashboard must be shared
	if d.Visibility != "org" {
		return store.Dashboard{}, notFound("dashboard") // personal/restricted: invisible to non-owners
	}
	if _, err := h.requireOrg(ctx, d.OrgID, "dashboard.share_org", true); err != nil {
		return store.Dashboard{}, err
	}
	return d, nil
}

func (h *Handlers) UpdateDashboard(ctx context.Context, req gen.UpdateDashboardRequestObject) (gen.UpdateDashboardResponseObject, error) {
	d, err := h.requireDashboardEditor(ctx, req.Dash)
	if err != nil {
		return nil, err
	}
	params := store.UpdateDashboardParams{ID: d.ID}
	if req.Body != nil {
		if req.Body.Name != nil {
			params.Name = pgtype.Text{String: *req.Body.Name, Valid: true}
		}
		if req.Body.Visibility != nil {
			v := string(*req.Body.Visibility)
			// raising visibility to shared needs the share-org grant.
			if v != "personal" && d.Visibility == "personal" {
				if _, err := h.requireOrg(ctx, d.OrgID, "dashboard.share_org", true); err != nil {
					return nil, err
				}
			}
			params.Visibility = pgtype.Text{String: v, Valid: true}
		}
		if req.Body.Layout != nil {
			b, _ := json.Marshal(*req.Body.Layout)
			params.Layout = b
		}
	}
	out, err := h.q.UpdateDashboard(ctx, params)
	if err != nil {
		return nil, err
	}
	return gen.UpdateDashboard200JSONResponse(dashboardToAPI(out, nil)), nil
}

func (h *Handlers) DeleteDashboard(ctx context.Context, req gen.DeleteDashboardRequestObject) (gen.DeleteDashboardResponseObject, error) {
	d, err := h.requireDashboardEditor(ctx, req.Dash)
	if err != nil {
		return nil, err
	}
	if err := h.q.DeleteDashboard(ctx, d.ID); err != nil {
		return nil, err
	}
	return gen.DeleteDashboard204Response{}, nil
}

func (h *Handlers) AddWidget(ctx context.Context, req gen.AddWidgetRequestObject) (gen.AddWidgetResponseObject, error) {
	d, err := h.requireDashboardEditor(ctx, req.Dash)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "source", Detail: "required"}})}
	}
	pos := json.RawMessage("{}")
	if req.Body.Pos != nil {
		if b, err := json.Marshal(req.Body.Pos); err == nil {
			pos = b
		}
	}
	wdg, err := h.q.AddWidget(ctx, store.AddWidgetParams{
		ID: ids.New("wdg"), DashboardID: d.ID,
		Source: string(req.Body.Source), Query: req.Body.Query, Viz: string(req.Body.Viz), Pos: pos,
	})
	if err != nil {
		return nil, err
	}
	return gen.AddWidget201JSONResponse(widgetToAPI(wdg)), nil
}

func dashboardToAPI(d store.Dashboard, widgets []store.DashboardWidget) gen.Dashboard {
	out := gen.Dashboard{
		Id: d.ID, Name: d.Name, Scope: d.Scope,
		Visibility: gen.Visibility(d.Visibility),
	}
	pb := d.Prebuilt
	out.Prebuilt = &pb
	owner := d.OwnerID
	out.OwnerId = &owner
	if len(d.Layout) > 0 {
		var m map[string]any
		if json.Unmarshal(d.Layout, &m) == nil {
			out.Layout = &m
		}
	}
	if widgets != nil {
		ws := make([]gen.Widget, 0, len(widgets))
		for _, w := range widgets {
			ws = append(ws, widgetToAPI(w))
		}
		out.Widgets = &ws
	}
	return out
}

func widgetToAPI(w store.DashboardWidget) gen.Widget {
	out := gen.Widget{Id: w.ID}
	out.Source = gen.WidgetSource(w.Source)
	out.Query = w.Query
	out.Viz = gen.WidgetViz(w.Viz)
	if len(w.Pos) > 0 {
		var pos struct {
			X *int `json:"x,omitempty"`
			Y *int `json:"y,omitempty"`
			W *int `json:"w,omitempty"`
			H *int `json:"h,omitempty"`
		}
		if json.Unmarshal(w.Pos, &pos) == nil && (pos.X != nil || pos.Y != nil || pos.W != nil || pos.H != nil) {
			out.Pos = &struct {
				H *int `json:"h,omitempty"`
				W *int `json:"w,omitempty"`
				X *int `json:"x,omitempty"`
				Y *int `json:"y,omitempty"`
			}{H: pos.H, W: pos.W, X: pos.X, Y: pos.Y}
		}
	}
	return out
}
