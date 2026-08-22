package provisioning

// T3.1: createEstimate — price a shape BEFORE anything provisions. The line
// grammar here is the invoice line grammar; acceptance (T3.3's createService)
// is the only path from an estimate to a running resource.

import (
	"context"

	"github.com/steloit/cloud/services/api/internal/estimates"
	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity"
	"github.com/steloit/cloud/services/api/internal/identity/session"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

func (h *Handlers) CreateEstimate(ctx context.Context, req gen.CreateEstimateRequestObject) (gen.CreateEstimateResponseObject, error) {
	p, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, identity.ErrNoSession
	}
	if req.Body == nil {
		return nil, problemError{p: problem.ValidationFailed([]problem.FieldError{{Field: "body", Detail: "required"}})}
	}
	// env context: resolve → org, membership-gated (404 — no id probing)
	orgID, envID := "", ""
	if req.Body.Env != nil && *req.Body.Env != "" {
		envID = *req.Body.Env
		resolved, err := h.svc.OrgForEnv(ctx, envID)
		if err != nil {
			return nil, notFound("environment")
		}
		if _, err := h.q.GetMemberRole(ctx, store.GetMemberRoleParams{OrgID: resolved, UserID: p.UserID}); err != nil {
			return nil, notFound("environment")
		}
		orgID = resolved
	}

	var shapes []estimates.ShapeInput
	if req.Body.TemplateId != nil && *req.Body.TemplateId != "" {
		// T12.4 estimate-at-consume: price the template's FROZEN contents — the
		// same integer cents shown at save and in list (ADR-021).
		tpl, err := h.q.GetTemplate(ctx, *req.Body.TemplateId)
		if err != nil {
			return nil, notFound("template")
		}
		tplOrg := tpl.OrgID
		if orgID != "" && orgID != tplOrg {
			return nil, notFound("template") // cross-org: indistinguishable from missing
		}
		if _, err := h.requireOrg(ctx, tplOrg, "template.consume", false); err != nil {
			return nil, notFound("template")
		}
		if tpl.Visibility == "restricted" && !h.canManageTemplates(ctx, tplOrg) {
			return nil, notFound("template")
		}
		tplShapes, _, err := TemplateShapes(tpl)
		if err != nil {
			return nil, err
		}
		shapes = tplShapes
		if orgID == "" {
			orgID = tplOrg
		}
	} else if req.Body.Services != nil {
		for _, s := range *req.Body.Services {
			in := estimates.ShapeInput{Product: string(s.Product)}
			if s.Intent != nil {
				in.Intent = string(*s.Intent)
			}
			if s.Name != nil {
				in.Name = *s.Name
			}
			if s.Shape != nil {
				in.Shape = *s.Shape
			}
			shapes = append(shapes, in)
		}
	}
	created, err := h.est.Create(ctx, orgID, envID, shapes)
	if err != nil {
		return nil, err
	}

	out := gen.Estimate{
		Id:                created.Row.ID,
		MonthlyTotalCents: int(created.Row.TotalCents),
	}
	exp := created.Row.ExpiresAt.Time
	out.ExpiresAt = &exp
	for _, l := range created.Lines {
		line := struct {
			Basis        *string      `json:"basis,omitempty"`
			EgressNote   *string      `json:"egress_note,omitempty"`
			Intent       *gen.Intent  `json:"intent,omitempty"`
			MonthlyCents *int         `json:"monthly_cents,omitempty"`
			Name         *string      `json:"name,omitempty"`
			Product      *gen.Product `json:"product,omitempty"`
		}{}
		basis, name, cents := l.Basis, l.Name, int(l.MonthlyCents.Int64())
		intent, product := gen.Intent(l.Intent), gen.Product(l.Product)
		line.Basis, line.Name, line.MonthlyCents = &basis, &name, &cents
		line.Intent, line.Product = &intent, &product
		line.EgressNote = l.EgressNote
		out.Lines = append(out.Lines, line)
	}
	return gen.CreateEstimate200JSONResponse(out), nil
}
