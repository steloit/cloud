package provisioning

// T12.4 templates (ADR-021): FROZEN COPIES, never live links. Capture builds
// contents from a WHITELIST only — service name/product/intent/shape/scaling
// and internal bindings by NAME — so secret material (provider_config,
// secret_ref, sec_ ids, ciphertext) is structurally unrepresentable; the
// templates table's CHECK constraint is the DB-level guard behind it (QA #5).
// Bindings whose target is outside the captured set become required_inputs —
// surfaced, never silently dropped. The instantiation estimate is priced at
// birth and re-priced on every contents change.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/estimates"
	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
	"github.com/steloit/cloud/services/api/internal/secrets"
)

// tplService is one captured service — whitelist fields ONLY. Adding a field
// here is a security decision (it ships to every template.consume holder).
type tplService struct {
	Name    string         `json:"name"`
	Product string         `json:"product"`
	Intent  string         `json:"intent,omitempty"`
	Shape   map[string]any `json:"shape"`
	Scaling map[string]any `json:"scaling,omitempty"`
}

// tplBinding is an INTERNAL binding (both ends captured), by NAME — ids are
// re-minted at instantiation (copies never link).
type tplBinding struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Scope  string `json:"scope"`
}

type tplContents struct {
	Services []tplService `json:"services"`
	Bindings []tplBinding `json:"bindings,omitempty"`
}

type tplRequiredInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CaptureTemplate snapshots a service subset of project/env into a frozen copy.
func (s *Service) CaptureTemplate(ctx context.Context, org store.Org, name, visibility, projectID, envID string, serviceIDs []string, actorID string) (store.Template, error) {
	prj, err := s.q.GetProject(ctx, projectID)
	if err != nil || prj.OrgID != org.ID {
		return store.Template{}, problemError{p: problem.NotFound("project")}
	}
	env, err := s.q.GetEnvironment(ctx, envID)
	if err != nil || env.ProjectID != prj.ID {
		return store.Template{}, problemError{p: problem.NotFound("environment")}
	}
	contents, required, cents, err := s.captureFrom(ctx, env.ID, serviceIDs)
	if err != nil {
		return store.Template{}, err
	}
	contentsJSON, _ := json.Marshal(contents)
	requiredJSON, _ := json.Marshal(required)
	if visibility == "" {
		visibility = "org"
	}
	row, err := s.q.InsertTemplate(ctx, store.InsertTemplateParams{
		ID: ids.New("tpl"), OrgID: org.ID, Name: name, Visibility: visibility,
		SourceProject: prj.Name, SourceEnv: env.Name,
		Contents: contentsJSON, RequiredInputs: requiredJSON,
		MonthlyEstimateCents: cents, UpdatedBy: actorID,
	})
	if err != nil {
		if isSecretGuardViolation(err) {
			return store.Template{}, problemError{p: problem.Conflict(
				[]string{"capture would include secret material (DB-level guard)"},
				"Remove secret-bearing fields from the source shapes and retry — templates never carry credentials (ADR-021).")}
		}
		return store.Template{}, err
	}
	s.record(ctx, events.Input{
		OrgID: org.ID, Kind: "lifecycle", Via: "user", Actor: actorID,
		Action: "template.captured", Subject: row.ID,
	})
	return row, nil
}

// captureFrom builds (contents, required_inputs, birth estimate) for an env's
// service subset — the whitelist walk shared by capture and refresh.
func (s *Service) captureFrom(ctx context.Context, envID string, serviceIDs []string) (tplContents, []tplRequiredInput, int64, error) {
	all, err := s.q.ListServicesForEnv(ctx, envID)
	if err != nil {
		return tplContents{}, nil, 0, err
	}
	want := map[string]bool{}
	for _, id := range serviceIDs {
		want[id] = true
	}
	captured := map[string]store.Service{} // id -> row (subset; empty selection = all)
	for _, svc := range all {
		if len(want) == 0 || want[svc.ID] {
			captured[svc.ID] = svc
		}
	}
	if len(captured) == 0 {
		return tplContents{}, nil, 0, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "services", Detail: "no captured services match this environment"}})}
	}

	var contents tplContents
	var shapes []estimates.ShapeInput
	for _, svc := range captured {
		var shape, scaling map[string]any
		_ = json.Unmarshal(svc.Shape, &shape)
		_ = json.Unmarshal(svc.Scaling, &scaling)
		contents.Services = append(contents.Services, tplService{
			Name: svc.Name, Product: svc.Product, Intent: svc.Intent.String,
			Shape: shape, Scaling: scaling,
		})
		shapes = append(shapes, estimates.ShapeInput{
			Product: svc.Product, Intent: svc.Intent.String, Name: svc.Name, Shape: shape,
		})
	}

	// Bindings: internal (both ends captured) ride along by NAME; anything else
	// becomes a required input — surfaced, never silently dropped (ADR-021).
	// provider_config/secret_ref are NEVER read here, by construction.
	var required []tplRequiredInput
	binds, err := s.q.ListBindingsForEnv(ctx, envID)
	if err != nil {
		return tplContents{}, nil, 0, err
	}
	for _, b := range binds {
		src, srcIn := captured[b.SourceID]
		if !srcIn {
			continue // source not captured — not this template's concern
		}
		if b.TargetType == "service" && b.TargetID.Valid {
			if tgt, tgtIn := captured[b.TargetID.String]; tgtIn {
				contents.Bindings = append(contents.Bindings, tplBinding{
					Source: src.Name, Target: tgt.Name, Scope: b.Scope,
				})
				continue
			}
		}
		// external provider or excluded service — a required input.
		desc := "binding from " + src.Name + " (" + b.TargetType + "); credentials re-mint per consumer"
		required = append(required, tplRequiredInput{
			Name: src.Name + "-" + b.TargetType, Description: desc,
		})
	}

	_, total, err := estimates.PriceAll(shapes)
	if err != nil {
		return tplContents{}, nil, 0, err
	}
	return contents, required, total, nil
}

// RefreshTemplate re-captures from the stored source names → a new version.
func (s *Service) RefreshTemplate(ctx context.Context, tpl store.Template, actorID string) (store.Template, error) {
	prj, err := s.q.ProjectByName(ctx, store.ProjectByNameParams{OrgID: tpl.OrgID, Name: tpl.SourceProject})
	if err != nil {
		return store.Template{}, problemError{p: problem.Conflict(
			[]string{"source project " + tpl.SourceProject + " no longer exists"},
			"The template remains usable as-is; refresh needs a live source (T2).")}
	}
	env, err := s.q.EnvByName(ctx, store.EnvByNameParams{ProjectID: prj.ID, Name: tpl.SourceEnv})
	if err != nil {
		return store.Template{}, problemError{p: problem.Conflict(
			[]string{"source environment " + tpl.SourceEnv + " no longer exists"},
			"The template remains usable as-is; refresh needs a live source (T2).")}
	}
	contents, required, cents, err := s.captureFrom(ctx, env.ID, nil) // full env on refresh
	if err != nil {
		return store.Template{}, err
	}
	contentsJSON, _ := json.Marshal(contents)
	requiredJSON, _ := json.Marshal(required)
	row, err := s.q.UpdateTemplate(ctx, store.UpdateTemplateParams{
		ID: tpl.ID, UpdatedBy: actorID,
		Contents: contentsJSON, RequiredInputs: requiredJSON,
		MonthlyEstimateCents: pgtype.Int8{Int64: cents, Valid: true},
	})
	if err != nil {
		return store.Template{}, err
	}
	s.record(ctx, events.Input{
		OrgID: tpl.OrgID, Kind: "lifecycle", Via: "user", Actor: actorID,
		Action: "template.refreshed", Subject: tpl.ID,
	})
	return row, nil
}

// TemplateShapes decodes a template's frozen contents into estimate inputs —
// the consume-side pricing path (createEstimate.template_id).
func TemplateShapes(tpl store.Template) ([]estimates.ShapeInput, tplContents, error) {
	var c tplContents
	if err := json.Unmarshal(tpl.Contents, &c); err != nil {
		return nil, c, fmt.Errorf("provisioning: template contents corrupt: %w", err)
	}
	shapes := make([]estimates.ShapeInput, 0, len(c.Services))
	for _, svc := range c.Services {
		shapes = append(shapes, estimates.ShapeInput{
			Product: svc.Product, Intent: svc.Intent, Name: svc.Name, Shape: svc.Shape,
		})
	}
	return shapes, c, nil
}

// InstantiateTemplate creates COPIES of a template's services (+ internal
// bindings, re-wired to the NEW ids) into a freshly created env. Estimate-
// before-provision holds per service (an internal estimate is created and
// consumed through the standard CreateService gate — so the budget cap
// enforces automatically); required_inputs must all be supplied or the whole
// instantiation refuses with every gap listed. Copies never link: nothing
// created here references the template row.
func (s *Service) InstantiateTemplate(ctx context.Context, est *estimates.Service, org store.Org, env store.Environment, tpl store.Template, inputs map[string]string, actorID string) error {
	shapes, contents, err := TemplateShapes(tpl)
	if err != nil {
		return err
	}
	// every required input must be wired — surfaced, never silently dropped.
	var required []tplRequiredInput
	_ = json.Unmarshal(tpl.RequiredInputs, &required)
	var missing []problem.FieldError
	for _, r := range required {
		if strings.TrimSpace(inputs[r.Name]) == "" {
			missing = append(missing, problem.FieldError{Field: "required_inputs." + r.Name, Detail: r.Description})
		}
	}
	if len(missing) > 0 {
		return problemError{p: problem.ValidationFailed(missing)}
	}

	// one estimate per service through the standard gate (F2 + the T11.6 cap).
	nameToID := map[string]string{}
	for i, sh := range shapes {
		row, err := est.Create(ctx, org.ID, env.ID, []estimates.ShapeInput{sh})
		if err != nil {
			return err
		}
		svcRow, err := s.CreateService(ctx, est, env, org.ID, CreateServiceInput{
			Name: sh.Name, Product: sh.Product, Intent: sh.Intent,
			Shape: sh.Shape, EstimateID: row.Row.ID,
		})
		if err != nil {
			return err
		}
		nameToID[contents.Services[i].Name] = svcRow.ID
	}
	// internal bindings, re-wired to the new ids.
	for _, b := range contents.Bindings {
		srcID, tgtID := nameToID[b.Source], nameToID[b.Target]
		if srcID == "" || tgtID == "" {
			continue // a binding referencing a non-instantiated service — skip, never invent
		}
		src, err := s.q.GetService(ctx, srcID)
		if err != nil {
			return err
		}
		if _, _, err := s.CreateBinding(ctx, src, tgtID, b.Scope, org.ID, env.ID, env.ProjectID, actorID); err != nil {
			return err
		}
	}
	// Required inputs are CREDENTIALS the consumer supplies (ADR-021:
	// "credentials re-mint per consumer") — each lands as a sealed secret in
	// the NEW env, named for the input. Never a live link back to the excluded
	// service, and never captured from the source.
	for _, r := range required {
		if _, err := s.vault.Put(ctx,
			secrets.Scope{OrgID: org.ID, ProjectID: env.ProjectID, EnvID: env.ID},
			r.Name, []byte(inputs[r.Name]), actorID); err != nil {
			return err
		}
	}
	_ = s.q.BumpTemplateUsage(ctx, tpl.ID) // informational only — never a link
	s.record(ctx, events.Input{
		OrgID: org.ID, Kind: "lifecycle", Via: "user", Actor: actorID,
		Action: "template.instantiated", Subject: tpl.ID,
	})
	return nil
}

// isSecretGuardViolation detects the templates_no_secret_material CHECK.
func isSecretGuardViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "templates_no_secret_material")
}

// GetTemplateFenced loads a template fenced to an org; restricted visibility is
// the caller's (handler's) concern.
func (s *Service) GetTemplateFenced(ctx context.Context, tplID, orgID string) (store.Template, error) {
	tpl, err := s.q.GetTemplate(ctx, tplID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && tpl.OrgID != orgID) {
		return store.Template{}, problemError{p: problem.NotFound("template")}
	}
	return tpl, err
}
