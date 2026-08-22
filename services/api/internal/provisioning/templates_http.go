package provisioning

// T12.4 template handlers (governance tag). Manage ops under template.manage;
// reads under template.consume; `restricted` visibility is readable only by
// managers. Template-by-id routes carry no org in the path — the template is
// loaded first and fenced to ITS org (a foreign id is indistinguishable from a
// missing one).

import (
	"context"
	"encoding/json"

	"errors"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/estimates"
	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/rbac"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// tplText / tplCents wrap optional narg params.
func tplText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }
func tplCents(v int64) pgtype.Int8 { return pgtype.Int8{Int64: v, Valid: true} }
func actorFrom(ctx context.Context) string {
	if p, ok := session.PrincipalFrom(ctx); ok {
		return p.UserID
	}
	return ""
}

func (h *Handlers) CaptureTemplate(ctx context.Context, req gen.CaptureTemplateRequestObject) (gen.CaptureTemplateResponseObject, error) {
	actor, err := h.requireOrg(ctx, req.OrgPathParam, "template.manage", true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || req.Body.Name == "" {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "name", Detail: "required"}})}
	}
	org, err := h.idsvc.GetOrg(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	visibility := ""
	if req.Body.Visibility != nil {
		visibility = string(*req.Body.Visibility)
		if visibility != "org" && visibility != "restricted" {
			return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "visibility", Detail: "one of org|restricted"}})}
		}
	}
	row, err := h.svc.CaptureTemplate(ctx, org, req.Body.Name, visibility,
		req.Body.Source.Project, req.Body.Source.Env, req.Body.Services, actor)
	if err != nil {
		return nil, err
	}
	return gen.CaptureTemplate201JSONResponse(templateToAPI(row)), nil
}

func (h *Handlers) ListTemplates(ctx context.Context, req gen.ListTemplatesRequestObject) (gen.ListTemplatesResponseObject, error) {
	if _, err := h.requireOrg(ctx, req.OrgPathParam, "template.consume", false); err != nil {
		return nil, err
	}
	rows, err := h.q.ListTemplates(ctx, req.OrgPathParam)
	if err != nil {
		return nil, err
	}
	canManage := h.canManageTemplates(ctx, req.OrgPathParam)
	out := make([]gen.Template, 0, len(rows))
	for _, t := range rows {
		if t.Visibility == "restricted" && !canManage {
			continue // restricted templates are visible to managers only
		}
		out = append(out, templateToAPI(t))
	}
	return gen.ListTemplates200JSONResponse(gen.TemplateList{Data: &out}), nil
}

func (h *Handlers) GetTemplate(ctx context.Context, req gen.GetTemplateRequestObject) (gen.GetTemplateResponseObject, error) {
	tpl, err := h.templateScoped(ctx, req.Tpl, "template.consume", false)
	if err != nil {
		return nil, err
	}
	if tpl.Visibility == "restricted" && !h.canManageTemplates(ctx, tpl.OrgID) {
		return nil, notFound("template") // restricted: invisible, not forbidden
	}
	return gen.GetTemplate200JSONResponse(templateToAPI(tpl)), nil
}

func (h *Handlers) UpdateTemplate(ctx context.Context, req gen.UpdateTemplateRequestObject) (gen.UpdateTemplateResponseObject, error) {
	tpl, err := h.templateScoped(ctx, req.Tpl, "template.manage", true)
	if err != nil {
		return nil, err
	}
	params := store.UpdateTemplateParams{ID: tpl.ID}
	params.UpdatedBy = actorFrom(ctx)
	if req.Body != nil {
		if req.Body.Name != nil {
			params.Name = tplText(*req.Body.Name)
		}
		if req.Body.Visibility != nil {
			v := string(*req.Body.Visibility)
			if v != "org" && v != "restricted" {
				return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "visibility", Detail: "one of org|restricted"}})}
			}
			params.Visibility = tplText(v)
		}
		if req.Body.Contents != nil {
			// contents edits re-run the WHOLE safety chain and — crucially —
			// what is stored is the RE-MARSHALED whitelist PROJECTION, never the
			// caller's raw bytes (review blocker: unknown keys must not persist
			// into an org-shared artifact). Shapes additionally project through
			// the closed per-product schema; then re-price; the DB CHECK still
			// guards at write.
			raw, _ := json.Marshal(*req.Body.Contents)
			shapes, decoded, err := TemplateShapes(store.Template{Contents: raw})
			if err != nil {
				return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "contents", Detail: "not a valid template contents object"}})}
			}
			for i := range decoded.Services {
				decoded.Services[i].Shape = estimates.ProjectShape(decoded.Services[i].Product, decoded.Services[i].Shape)
				decoded.Services[i].Scaling = projectScaling(decoded.Services[i].Scaling)
				shapes[i].Shape = decoded.Services[i].Shape
			}
			_, total, err := estimates.PriceAll(shapes)
			if err != nil {
				return nil, err
			}
			params.MonthlyEstimateCents = tplCents(total.Int64())
			projected, _ := json.Marshal(decoded)
			params.Contents = projected
		}
	}
	row, err := h.q.UpdateTemplate(ctx, params)
	if err != nil {
		return nil, mapTemplateWriteError(err)
	}
	return gen.UpdateTemplate200JSONResponse(templateToAPI(row)), nil
}

func (h *Handlers) DeleteTemplate(ctx context.Context, req gen.DeleteTemplateRequestObject) (gen.DeleteTemplateResponseObject, error) {
	tpl, err := h.templateScoped(ctx, req.Tpl, "template.manage", true)
	if err != nil {
		return nil, err
	}
	// ADR-021: deleting the recipe doesn't touch the meals.
	if _, err := h.q.DeleteTemplate(ctx, store.DeleteTemplateParams{ID: tpl.ID, OrgID: tpl.OrgID}); err != nil {
		return nil, err
	}
	return gen.DeleteTemplate204Response{}, nil
}

func (h *Handlers) RefreshTemplate(ctx context.Context, req gen.RefreshTemplateRequestObject) (gen.RefreshTemplateResponseObject, error) {
	tpl, err := h.templateScoped(ctx, req.Tpl, "template.manage", true)
	if err != nil {
		return nil, err
	}
	row, err := h.svc.RefreshTemplate(ctx, tpl, actorFrom(ctx))
	if err != nil {
		return nil, err
	}
	return gen.RefreshTemplate200JSONResponse(templateToAPI(row)), nil
}

// templateScoped loads a template by id and authorizes perm in ITS org.
func (h *Handlers) templateScoped(ctx context.Context, tplID string, perm string, mutating bool) (store.Template, error) {
	tpl, err := h.q.GetTemplate(ctx, tplID)
	if err != nil {
		return store.Template{}, notFound("template")
	}
	if _, err := h.requireOrg(ctx, tpl.OrgID, rbac.Permission(perm), mutating); err != nil {
		if errors.Is(err, identity.ErrNoSession) {
			return store.Template{}, err // unauthenticated is a 401, never a 404
		}
		return store.Template{}, notFound("template") // foreign org: indistinguishable from missing
	}
	return tpl, nil
}

// canManageTemplates reports whether the caller holds template.manage (used to
// gate restricted visibility on read paths — a denial here is a filter, not an
// error).
func (h *Handlers) canManageTemplates(ctx context.Context, orgID string) bool {
	_, err := h.requireOrg(ctx, orgID, "template.manage", false)
	return err == nil
}

func templateToAPI(t store.Template) gen.Template {
	out := gen.Template{
		Id: t.ID, Name: t.Name, Version: int(t.Version),
		Visibility: gen.TemplateVisibility(t.Visibility),
	}
	cents := int(t.MonthlyEstimateCents)
	out.MonthlyEstimateCents = &cents
	used := int(t.UsedByCount)
	out.UsedByCount = &used
	by := t.UpdatedBy
	out.UpdatedBy = &by
	at := t.UpdatedAt.Time
	out.UpdatedAt = &at
	prj, env := t.SourceProject, t.SourceEnv
	out.Source = &struct {
		Env     *string `json:"env,omitempty"`
		Project *string `json:"project,omitempty"`
	}{Env: &env, Project: &prj}
	var contents map[string]interface{}
	if json.Unmarshal(t.Contents, &contents) == nil {
		out.Contents = &contents
	}
	var required []struct {
		Description *string `json:"description,omitempty"`
		Name        *string `json:"name,omitempty"`
	}
	var raw []tplRequiredInput
	if json.Unmarshal(t.RequiredInputs, &raw) == nil {
		for _, r := range raw {
			r := r
			required = append(required, struct {
				Description *string `json:"description,omitempty"`
				Name        *string `json:"name,omitempty"`
			}{Description: &r.Description, Name: &r.Name})
		}
		out.RequiredInputs = &required
	}
	return out
}
