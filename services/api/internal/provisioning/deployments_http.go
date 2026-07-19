package provisioning

import (
	"context"
	"encoding/json"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

func deploymentToAPI(d store.Deployment) gen.Deployment {
	out := gen.Deployment{
		Id: d.ID, EnvId: d.EnvID, ServiceId: d.ServiceID,
		State: gen.DeploymentState(d.State), Actor: d.Actor,
	}
	n := int(d.Number)
	out.Number = &n
	if d.GitSha != "" {
		sha := d.GitSha
		out.GitSha = &sha
	}
	if d.CanaryPercent.Valid {
		cp := int(d.CanaryPercent.Int32)
		out.CanaryPercent = &cp
	}
	if d.Annotation.Valid {
		a := d.Annotation.String
		out.Annotation = &a
	}
	var gates []struct {
		Detail *string                    `json:"detail,omitempty"`
		Name   *string                    `json:"name,omitempty"`
		Status *gen.DeploymentGatesStatus `json:"status,omitempty"`
	}
	var rawGates []struct{ Name, Status, Detail string }
	if json.Unmarshal(d.Gates, &rawGates) == nil {
		for i := range rawGates {
			name, detail := rawGates[i].Name, rawGates[i].Detail
			status := gen.DeploymentGatesStatus(rawGates[i].Status)
			gates = append(gates, struct {
				Detail *string                    `json:"detail,omitempty"`
				Name   *string                    `json:"name,omitempty"`
				Status *gen.DeploymentGatesStatus `json:"status,omitempty"`
			}{Name: &name, Status: &status, Detail: &detail})
		}
	}
	out.Gates = &gates
	created := d.CreatedAt.Time
	out.CreatedAt = &created
	return out
}

func (h *Handlers) CreateDeployment(ctx context.Context, req gen.CreateDeploymentRequestObject) (gen.CreateDeploymentResponseObject, error) {
	env, orgID, err := h.envScoped(ctx, req.EnvPathParam)
	if err != nil {
		return nil, err
	}
	actor, err := h.requireOrg(ctx, orgID, "deploy.promote", true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || req.Body.Service == nil || *req.Body.Service == "" {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "service", Detail: "required"}})}
	}
	in := CreateDeploymentInput{ServiceID: *req.Body.Service, ActorID: actor}
	if req.Body.GitSha != nil {
		in.GitSha = *req.Body.GitSha
	}
	if req.Body.PromoteFrom != nil {
		in.PromoteFrom = *req.Body.PromoteFrom
	}
	row, err := h.svc.CreateDeployment(ctx, env, orgID, in)
	if err != nil {
		return nil, err
	}
	return gen.CreateDeployment201JSONResponse(deploymentToAPI(row)), nil
}

func (h *Handlers) ListDeployments(ctx context.Context, req gen.ListDeploymentsRequestObject) (gen.ListDeploymentsResponseObject, error) {
	env, _, err := h.envScoped(ctx, req.EnvPathParam)
	if err != nil {
		return nil, err
	}
	rows, err := h.svc.ListDeployments(ctx, env.ID)
	if err != nil {
		return nil, err
	}
	data := make([]gen.Deployment, 0, len(rows))
	for _, d := range rows {
		data = append(data, deploymentToAPI(d))
	}
	return gen.ListDeployments200JSONResponse(gen.DeploymentList{Data: &data}), nil
}

func (h *Handlers) RollbackDeployment(ctx context.Context, req gen.RollbackDeploymentRequestObject) (gen.RollbackDeploymentResponseObject, error) {
	dep, env, orgID, err := h.svc.DeploymentOrg(ctx, req.Dep)
	if err != nil {
		return nil, err
	}
	if _, err := h.memberOrg(ctx, orgID); err != nil {
		return nil, notFound("deployment")
	}
	actor, err := h.requireOrg(ctx, orgID, "deploy.rollback", true)
	if err != nil {
		return nil, err
	}
	row, err := h.svc.Rollback(ctx, dep, env, orgID, actor)
	if err != nil {
		return nil, err
	}
	return gen.RollbackDeployment201JSONResponse(deploymentToAPI(row)), nil
}
