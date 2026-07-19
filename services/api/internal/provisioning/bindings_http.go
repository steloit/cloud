package provisioning

// T3.6 strict handlers. Reads always mask env-var values; secret_ref is
// never returned (the schema documents it; we simply never populate it).

import (
	"context"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

func bindingToAPI(b store.Binding, envVarName string) gen.Binding {
	out := gen.Binding{
		Id: b.ID, SourceId: b.SourceID,
		Scope:      gen.BindingScope(b.Scope),
		Status:     gen.BindingStatus(b.Status),
		TargetType: gen.BindingTargetType(b.TargetType),
	}
	if b.TargetID.Valid {
		tid := b.TargetID.String
		out.TargetId = &tid
	}
	if b.Intent.Valid {
		intent := gen.Intent(b.Intent.String)
		out.Intent = &intent
	}
	if b.RotatedAt.Valid {
		t := b.RotatedAt.Time
		out.RotatedAt = &t
	}
	if envVarName != "" {
		vars := map[string]string{envVarName: "••• masked — injected at deploy"}
		out.EnvVars = &vars
	}
	return out
}

func (h *Handlers) CreateBinding(ctx context.Context, req gen.CreateBindingRequestObject) (gen.CreateBindingResponseObject, error) {
	source, orgID, err := h.serviceScoped(ctx, req.ServicePathParam)
	if err != nil {
		return nil, err
	}
	actor, err := h.requireOrg(ctx, orgID, "binding.manage", true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || req.Body.Target == "" {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "target", Detail: "required"}})}
	}
	scope := "read_only"
	if req.Body.Scope != nil {
		scope = string(*req.Body.Scope)
	}
	env, err := h.q.GetEnvironment(ctx, source.EnvID)
	if err != nil {
		return nil, err
	}
	row, envVar, err := h.svc.CreateBinding(ctx, source, req.Body.Target, scope, orgID, env.ID, env.ProjectID, actor)
	if err != nil {
		return nil, err
	}
	return gen.CreateBinding201JSONResponse(bindingToAPI(row, envVar)), nil
}

func (h *Handlers) ListBindings(ctx context.Context, req gen.ListBindingsRequestObject) (gen.ListBindingsResponseObject, error) {
	source, _, err := h.serviceScoped(ctx, req.ServicePathParam)
	if err != nil {
		return nil, err
	}
	rows, err := h.svc.ListBindings(ctx, source.ID)
	if err != nil {
		return nil, err
	}
	data := make([]gen.Binding, 0, len(rows))
	for _, b := range rows {
		envVar := ""
		if b.TargetID.Valid {
			if target, err := h.q.GetService(ctx, b.TargetID.String); err == nil {
				envVar = EnvVarName(target.Name)
			}
		}
		data = append(data, bindingToAPI(b, envVar))
	}
	return gen.ListBindings200JSONResponse(gen.BindingList{Data: &data}), nil
}

func (h *Handlers) DeleteBinding(ctx context.Context, req gen.DeleteBindingRequestObject) (gen.DeleteBindingResponseObject, error) {
	b, source, orgID, err := h.svc.BindingOrg(ctx, req.Binding)
	if err != nil {
		return nil, err
	}
	if _, err := h.memberOrg(ctx, orgID); err != nil {
		return nil, notFound("binding")
	}
	actor, err := h.requireOrg(ctx, orgID, "binding.manage", true)
	if err != nil {
		return nil, err
	}
	env, err := h.q.GetEnvironment(ctx, source.EnvID)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteBinding(ctx, b, orgID, env.ID, env.ProjectID, actor); err != nil {
		return nil, err
	}
	return gen.DeleteBinding204Response{}, nil
}
