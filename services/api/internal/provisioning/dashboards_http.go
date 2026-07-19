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
	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity"
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
	if req.Body == nil || req.Body.Name == "" || len(req.Body.Name) > 200 {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "name", Detail: "required, 1–200 chars"}})}
	}
	scope := req.Body.Scope
	if scope != "org" && scopeProject(scope) == "" {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "scope", Detail: `must be "org" or "project:prj_…"`}})}
	}
	vis := string(req.Body.Visibility)
	if !validVisibility(vis) {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "visibility", Detail: "personal|org|restricted"}})}
	}

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
// Returns the dashboard and the acting user id.
func (h *Handlers) requireDashboardEditor(ctx context.Context, dashID string) (store.Dashboard, string, error) {
	d, err := h.q.GetDashboard(ctx, dashID)
	if err != nil {
		return store.Dashboard{}, "", notFound("dashboard")
	}
	p, err := h.memberOrg(ctx, d.OrgID)
	if err != nil {
		return store.Dashboard{}, "", notFound("dashboard")
	}
	// Mutating with a limited-scope token is refused on EVERY edit path — the
	// owner path must not skip it (review H1: an owner's read_only token could
	// otherwise PATCH/DELETE/add-widget).
	if p.Kind == "token" && p.Scope != "full" {
		return store.Dashboard{}, "", identity.ErrScopeDenied
	}
	// The SCOPE born-filter binds on the editor path too, not only reads
	// (review: defense-in-depth + a scoped-project-deleted dashboard must not be
	// editable when it is no longer readable).
	if prj := scopeProject(d.Scope); prj != "" {
		if _, _, err := h.projectScoped(ctx, prj); err != nil {
			return store.Dashboard{}, "", notFound("dashboard")
		}
	}
	// Prebuilt dashboards are GENERATED VIEWS, not files (Design-Spec §244): they
	// are always-current and immutable — never edited or deleted, only forked
	// ("Customize"). Refuse every mutation with the remediation that names fork.
	if d.Prebuilt {
		return store.Dashboard{}, "", problemError{p: problem.Conflict(
			[]string{"prebuilt dashboards are generated views and cannot be edited"},
			"Fork it (Customize) to get an editable copy in My dashboards.")}
	}
	if d.OwnerID == p.UserID {
		return d, p.UserID, nil
	}
	// not the owner → needs the share-org grant AND the dashboard must be shared
	if d.Visibility != "org" {
		return store.Dashboard{}, "", notFound("dashboard") // personal/restricted: invisible to non-owners
	}
	if _, err := h.requireOrg(ctx, d.OrgID, "dashboard.share_org", true); err != nil {
		return store.Dashboard{}, "", err
	}
	return d, p.UserID, nil
}

func (h *Handlers) UpdateDashboard(ctx context.Context, req gen.UpdateDashboardRequestObject) (gen.UpdateDashboardResponseObject, error) {
	d, actor, err := h.requireDashboardEditor(ctx, req.Dash)
	if err != nil {
		return nil, err
	}
	params := store.UpdateDashboardParams{ID: d.ID}
	if req.Body != nil {
		if req.Body.Name != nil {
			if len(*req.Body.Name) == 0 || len(*req.Body.Name) > 200 {
				return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "name", Detail: "1–200 chars"}})}
			}
			params.Name = pgtype.Text{String: *req.Body.Name, Valid: true}
		}
		if req.Body.Visibility != nil {
			v := string(*req.Body.Visibility)
			if !validVisibility(v) {
				return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "visibility", Detail: "personal|org|restricted"}})}
			}
			// Any transition into OR out of the shared (org) state is a
			// governance change → needs the share-org grant (review: a share_org
			// re-check that only fired on personal→X let restricted→org and
			// org→personal slip through). personal↔restricted is ungated while
			// restricted is owner-only; add the gate when a restricted-share
			// surface lands (review F8, recorded).
			if v != d.Visibility && (v == "org" || d.Visibility == "org") {
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
	// F7: an edit to a shared dashboard is live for all viewers and AUDITED —
	// including an UN-share (org→personal), the most governance-sensitive edit,
	// so audit on either the pre- OR post-state being shared (review F6).
	if d.Visibility == "org" || out.Visibility == "org" {
		h.svc.record(ctx, events.Input{
			OrgID: out.OrgID, Kind: "lifecycle", Via: "user", Actor: actor,
			Action: "dashboard.updated", Subject: out.ID,
		})
	}
	return gen.UpdateDashboard200JSONResponse(dashboardToAPI(out, nil)), nil
}

func (h *Handlers) DeleteDashboard(ctx context.Context, req gen.DeleteDashboardRequestObject) (gen.DeleteDashboardResponseObject, error) {
	d, actor, err := h.requireDashboardEditor(ctx, req.Dash)
	if err != nil {
		return nil, err
	}
	if err := h.q.DeleteDashboard(ctx, d.ID); err != nil {
		return nil, err
	}
	if d.Visibility == "org" {
		h.svc.record(ctx, events.Input{
			OrgID: d.OrgID, Kind: "lifecycle", Via: "user", Actor: actor,
			Action: "dashboard.deleted", Subject: d.ID,
		})
	}
	return gen.DeleteDashboard204Response{}, nil
}

func (h *Handlers) AddWidget(ctx context.Context, req gen.AddWidgetRequestObject) (gen.AddWidgetResponseObject, error) {
	d, _, err := h.requireDashboardEditor(ctx, req.Dash)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "source", Detail: "required"}})}
	}
	// Validate the closed enums at the boundary → a clean 422, never a raw DB
	// CHECK violation surfacing as 500 (review).
	var fe []problem.FieldError
	if !req.Body.Source.Valid() {
		fe = append(fe, problem.FieldError{Field: "source", Detail: "metrics|logs|cost|deploys|ai|alerts"})
	}
	if !req.Body.Viz.Valid() {
		fe = append(fe, problem.FieldError{Field: "viz", Detail: "line|area|stat|table|list"})
	}
	if req.Body.Query == "" || len(req.Body.Query) > 4096 {
		fe = append(fe, problem.FieldError{Field: "query", Detail: "1–4096 chars"})
	}
	if len(fe) > 0 {
		return nil, problemError{p: problem.ValidationFailed(fe)}
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

func validVisibility(v string) bool {
	return v == "personal" || v == "org" || v == "restricted"
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

// copyToPersonal is the shared fork/duplicate engine (spec §2c, Design-Spec
// §244: prebuilt dashboards are views, not files — "Customize" forks a copy
// into My dashboards with template copy-semantics). The source must be READABLE
// by the caller (both axes); the copy is a NEW personal dashboard owned by the
// caller, with the source's scope, layout, and widgets duplicated. The source
// is never mutated (a prebuilt stays read-only and always-current).
func (h *Handlers) copyToPersonal(ctx context.Context, dashID, nameSuffix string) (gen.Dashboard, error) {
	src, p, err := h.dashboardReadable(ctx, dashID)
	if err != nil {
		return gen.Dashboard{}, err
	}
	// A project-scoped source stays project-scoped in the copy — the born-filter
	// travels with it (the caller already proved project access via readable).
	copyName := src.Name
	if nameSuffix != "" {
		copyName = src.Name + nameSuffix
	}
	dsh, err := h.q.CreateDashboard(ctx, store.CreateDashboardParams{
		ID: ids.New("dsh"), OrgID: src.OrgID, Name: copyName,
		Scope: src.Scope, Visibility: "personal", OwnerID: p.UserID, Layout: src.Layout,
	})
	if err != nil {
		return gen.Dashboard{}, err
	}
	widgets, err := h.q.ListWidgetsForDashboard(ctx, src.ID)
	if err != nil {
		return gen.Dashboard{}, err
	}
	copied := make([]store.DashboardWidget, 0, len(widgets))
	for _, w := range widgets {
		nw, err := h.q.AddWidget(ctx, store.AddWidgetParams{
			ID: ids.New("wdg"), DashboardID: dsh.ID,
			Source: w.Source, Query: w.Query, Viz: w.Viz, Pos: w.Pos,
		})
		if err != nil {
			return gen.Dashboard{}, err
		}
		copied = append(copied, nw)
	}
	return dashboardToAPI(dsh, copied), nil
}

func (h *Handlers) ForkDashboard(ctx context.Context, req gen.ForkDashboardRequestObject) (gen.ForkDashboardResponseObject, error) {
	out, err := h.copyToPersonal(ctx, req.Dash, " (customized)")
	if err != nil {
		return nil, err
	}
	return gen.ForkDashboard201JSONResponse(out), nil
}

func (h *Handlers) DuplicateDashboard(ctx context.Context, req gen.DuplicateDashboardRequestObject) (gen.DuplicateDashboardResponseObject, error) {
	out, err := h.copyToPersonal(ctx, req.Dash, " (copy)")
	if err != nil {
		return nil, err
	}
	return gen.DuplicateDashboard201JSONResponse(out), nil
}

func (h *Handlers) DeleteWidget(ctx context.Context, req gen.DeleteWidgetRequestObject) (gen.DeleteWidgetResponseObject, error) {
	d, actor, err := h.requireDashboardEditor(ctx, req.Dash)
	if err != nil {
		return nil, err
	}
	// widget must belong to this dashboard (no cross-dashboard delete).
	if _, err := h.q.GetWidget(ctx, store.GetWidgetParams{ID: req.Wdg, DashboardID: d.ID}); err != nil {
		return nil, notFound("widget")
	}
	if err := h.q.DeleteWidget(ctx, store.DeleteWidgetParams{ID: req.Wdg, DashboardID: d.ID}); err != nil {
		return nil, err
	}
	if d.Visibility == "org" {
		h.svc.record(ctx, events.Input{
			OrgID: d.OrgID, Kind: "lifecycle", Via: "user", Actor: actor,
			Action: "dashboard.updated", Subject: d.ID,
		})
	}
	return gen.DeleteWidget204Response{}, nil
}
