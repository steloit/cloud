package provisioning

// T3.3 strict handlers: the service surface. The estimate gate lives in the
// service layer; handlers translate and authorize.

import (
	"context"
	"encoding/json"
	"time"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

func serviceToAPI(s store.Service) gen.Service {
	out := gen.Service{Id: s.ID, EnvId: s.EnvID, Name: s.Name,
		Product: gen.Product(s.Product), Status: gen.ServiceStatus(s.Status)}
	if s.Intent.Valid {
		intent := gen.Intent(s.Intent.String)
		out.Intent = &intent
	}
	var shape map[string]any
	if json.Unmarshal(s.Shape, &shape) == nil {
		out.Shape = &shape
	}
	cents := int(s.MonthlyEstimateCents)
	out.MonthlyEstimateCents = &cents
	var steps []struct {
		Status *gen.ServiceProvisioningStepsStatus `json:"status,omitempty"`
		Step   *string                             `json:"step,omitempty"`
	}
	var raw []struct{ Step, Status string }
	if json.Unmarshal(s.ProvisioningSteps, &raw) == nil {
		for i := range raw {
			st := gen.ServiceProvisioningStepsStatus(raw[i].Status)
			sp := raw[i].Step
			steps = append(steps, struct {
				Status *gen.ServiceProvisioningStepsStatus `json:"status,omitempty"`
				Step   *string                             `json:"step,omitempty"`
			}{Status: &st, Step: &sp})
		}
	}
	out.ProvisioningSteps = &steps
	created := s.CreatedAt.Time
	out.CreatedAt = &created
	return out
}

// envScoped resolves env → org and 404s non-members.
func (h *Handlers) envScoped(ctx context.Context, envID string) (store.Environment, string, error) {
	orgID, err := h.svc.OrgForEnv(ctx, envID)
	if err != nil {
		return store.Environment{}, "", notFound("environment")
	}
	if _, err := h.memberOrg(ctx, orgID); err != nil {
		return store.Environment{}, "", notFound("environment")
	}
	env, err := h.q.GetEnvironment(ctx, envID)
	if err != nil {
		return store.Environment{}, "", err
	}
	return env, orgID, nil
}

func (h *Handlers) CreateService(ctx context.Context, req gen.CreateServiceRequestObject) (gen.CreateServiceResponseObject, error) {
	env, orgID, err := h.envScoped(ctx, req.EnvPathParam)
	if err != nil {
		return nil, err
	}
	actor, err := h.requireOrg(ctx, orgID, "service.create", true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "body", Detail: "required"}})}
	}
	if req.Body.Bindings != nil && len(*req.Body.Bindings) > 0 {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{
			{Field: "bindings", Detail: "pre-wired bindings arrive with T3.6 — create the service, then bind"}})}
	}
	in := CreateServiceInput{
		Name: req.Body.Name, Product: string(req.Body.Product),
		EstimateID: req.Body.EstimateId, ActorID: actor,
	}
	if req.Body.Intent != nil {
		in.Intent = string(*req.Body.Intent)
	}
	if req.Body.Shape != nil {
		in.Shape = *req.Body.Shape
	}
	row, err := h.svc.CreateService(ctx, h.est, env, orgID, in)
	if err != nil {
		return nil, err
	}
	return gen.CreateService201JSONResponse(serviceToAPI(row)), nil
}

func (h *Handlers) ListServices(ctx context.Context, req gen.ListServicesRequestObject) (gen.ListServicesResponseObject, error) {
	env, _, err := h.envScoped(ctx, req.EnvPathParam)
	if err != nil {
		return nil, err
	}
	rows, err := h.svc.ListServices(ctx, env.ID)
	if err != nil {
		return nil, err
	}
	data := make([]gen.Service, 0, len(rows))
	for _, r := range rows {
		data = append(data, serviceToAPI(r))
	}
	return gen.ListServices200JSONResponse(gen.ServiceList{Data: &data}), nil
}

// serviceScoped resolves service → org, 404s non-members.
func (h *Handlers) serviceScoped(ctx context.Context, serviceID string) (store.Service, string, error) {
	svc, orgID, err := h.svc.ServiceOrg(ctx, serviceID)
	if err != nil {
		return store.Service{}, "", err
	}
	if _, err := h.memberOrg(ctx, orgID); err != nil {
		return store.Service{}, "", notFound("service")
	}
	return svc, orgID, nil
}

func (h *Handlers) GetService(ctx context.Context, req gen.GetServiceRequestObject) (gen.GetServiceResponseObject, error) {
	svc, _, err := h.serviceScoped(ctx, req.ServicePathParam)
	if err != nil {
		return nil, err
	}
	return gen.GetService200JSONResponse(serviceToAPI(svc)), nil
}

func (h *Handlers) UpdateService(ctx context.Context, req gen.UpdateServiceRequestObject) (gen.UpdateServiceResponseObject, error) {
	svc, orgID, err := h.serviceScoped(ctx, req.ServicePathParam)
	if err != nil {
		return nil, err
	}
	actor, err := h.requireOrg(ctx, orgID, "service.scale", true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "body", Detail: "required"}})}
	}
	var shape map[string]any
	if req.Body.Shape != nil {
		shape = *req.Body.Shape
	}
	var scaling, override []byte
	if req.Body.Scaling != nil {
		scaling, _ = json.Marshal(req.Body.Scaling)
	}
	if req.Body.Override != nil {
		if req.Body.Override.Reason == "" {
			return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{
				{Field: "override.reason", Detail: "required — manual pins carry a reason and auto-expire in 24h (D22)"}})}
		}
		// A pin below one instance is REFUSED, not silently ignored.
		//
		// Without this it returned 200 and did nothing visible: overrideInstances
		// declines it, no pin reaches the desired doc, the price stays at base —
		// but the raw pin is still written to `override`, where it is non-NULL,
		// unhonoured, and NOT SWEPT, because the expires_at stamped just below is
		// future and well-formed so every arm of ListExpiredOverrides calls it
		// live. A column holding a pin nothing will honour and nothing will clear,
		// for 24h, after a 200. The same value in `shape.instances` is a 422 from
		// the pricing engine — one class of input, two answers, which is the
		// defect this task already fixed one field over (services.go's Resolve).
		if req.Body.Override.Instances < 1 {
			return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{
				{Field: "override.instances", Detail: "must be at least 1"}})}
		}
		override, _ = json.Marshal(map[string]any{
			"instances":  req.Body.Override.Instances,
			"reason":     req.Body.Override.Reason,
			"expires_at": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		})
	}
	row, err := h.svc.UpdateService(ctx, svc, orgID, actor, shape, scaling, override)
	if err != nil {
		return nil, err
	}
	return gen.UpdateService200JSONResponse(serviceToAPI(row)), nil
}

func (h *Handlers) DeleteService(ctx context.Context, req gen.DeleteServiceRequestObject) (gen.DeleteServiceResponseObject, error) {
	svc, orgID, err := h.serviceScoped(ctx, req.ServicePathParam)
	if err != nil {
		return nil, err
	}
	actor, err := h.requireOrg(ctx, orgID, "service.delete", true)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteService(ctx, svc, orgID, actor); err != nil {
		return nil, err
	}
	return gen.DeleteService202Response{}, nil
}
