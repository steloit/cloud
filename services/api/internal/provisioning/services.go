package provisioning

// T3.3: service rows + the guarded status machine (ADR-024 vocabulary:
// provisioning|ready|degraded|failed|suspended|deleting — `ready`, never
// `running`; metering starts at ready). Rows are DESIRED STATE (D9): the
// reconciler (T3.4+, cell-agent) converges actual state; nothing here talks
// to infrastructure.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/estimates"
	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/metering"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// enforceBudget is the hard spend cap (T11.6, F9): before an estimate is
// accepted, project the org's committed monthly run-rate (plan fee + Σ active
// services' monthly estimates) plus this service's monthly cost against the
// bound. Over the bound → refuse with the arithmetic shown. Uncapped (no row or
// a null limit) → always proceeds. Running services are NEVER touched — the cap
// pauses new provisioning only (US-11.5: cancel/pause ≠ delete).
func (s *Service) enforceBudget(ctx context.Context, orgID string, newMonthlyCents int64) error {
	budget, err := s.q.GetBudget(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) || !budget.LimitCents.Valid {
		return nil // uncapped
	}
	if err != nil {
		return err
	}
	limit := budget.LimitCents.Int64
	org, err := s.q.GetOrg(ctx, orgID)
	if err != nil {
		return err
	}
	var planFee int64
	if fee, ok := s.plans.PlanFeeCents(org.Plan); ok {
		planFee = int64(fee)
	}
	committed, err := s.q.SumOrgMonthlyEstimate(ctx, orgID)
	if err != nil {
		return err
	}
	current := planFee + committed
	projected := current + newMonthlyCents
	if projected > limit {
		// F9 flagship: an ENFORCED bound, refused at accept time (402) with the
		// arithmetic shown — never an alert-only. Every cap hit lands on the
		// events spine (AC3) so "the cap is real" is auditable, not just a UI toast.
		s.record(ctx, events.Input{
			OrgID: orgID, Kind: "billing", Via: "system", Actor: "system",
			Action: "billing.spend_cap_reached", Subject: orgID,
			Detail: []byte(fmt.Sprintf(`{"cap_cents":%d,"current_cents":%d,"requested_cents":%d,"projected_cents":%d}`,
				limit, current, newMonthlyCents, projected)),
		})
		// The hard spend cap is a 402 quota_exceeded (the x-error-catalog's
		// sanctioned "hard quota: fails with remediation" — NOT a new error
		// class): an ENFORCED bound refused at accept time with the arithmetic,
		// never an alert-only.
		return problemError{p: problem.QuotaHard(
			fmt.Sprintf("this raises your monthly spend to %s (current %s + this service %s), above your %s cap",
				dollars(projected), dollars(current), dollars(newMonthlyCents), dollars(limit)),
			"Raise the budget in Billing, or provision a smaller shape — nothing running is affected.")}
	}
	return nil
}

// dollars renders integer cents as $D.CC for the cap's shown arithmetic.
func dollars(cents int64) string {
	return fmt.Sprintf("$%d.%02d", cents/100, ((cents%100)+100)%100)
}

// transitions is the closed status machine. deleting is terminal (the
// reconciler removes the row after teardown + final backup).
var transitions = map[string][]string{
	"provisioning": {"ready", "failed", "deleting"}, // deleting = cancel-the-create
	"ready":        {"degraded", "suspended", "deleting"},
	"degraded":     {"ready", "failed", "deleting"},
	"failed":       {"provisioning", "deleting"}, // retry re-provisions
	"suspended":    {"ready", "deleting"},
	"deleting":     {},
}

// CanTransition reports whether from → to is a legal edge.
func CanTransition(from, to string) bool {
	for _, t := range transitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

// desiredDoc builds the reconciler's desired-state document (US-1.3a) — what the
// cell-agent renders from (e1-substrate-design.md §2: product + intent + shape +
// lifecycle flags). Substrate names never appear here (D8); this is grammar
// only. `deleting` marks a teardown so the cell converges the service to gone.
// `namespace` is the env-derived cell namespace (ADR-0012).
func desiredDoc(product, intent, namespace string, shape, scaling, override []byte, deleting bool) []byte {
	doc := map[string]any{"product": product}
	if namespace != "" {
		doc["namespace"] = namespace // the cell renders here (env-derived, ADR-0012)
	}
	if intent != "" {
		doc["intent"] = intent
	}
	embed := func(key string, raw []byte) {
		if len(raw) > 0 {
			var v any
			if json.Unmarshal(raw, &v) == nil && v != nil {
				doc[key] = v
			}
		}
	}
	embed("shape", shape)
	embed("scaling", scaling)
	// override is the manual instance-pin {instances, reason, expires_at} — a
	// load-bearing capacity input (count), so the cell must render from it.
	embed("override", override)
	if deleting {
		doc["deleting"] = true
	}
	b, _ := json.Marshal(doc)
	return b
}

// resolveNamespace derives the cell namespace for an environment.
//
// It is derived from the environment's ID, NOT from project/env NAMES. Names are
// unique only PER ORG (projects: UNIQUE (org_id, name)), so `proj--env` puts two
// different orgs' `api`/`prod` in the SAME namespace — sharing the tenant
// isolation boundary D7 defines (default-deny NetworkPolicy, ResourceQuota, and
// CNPG's generated `<cluster>-app` credential Secrets). Ids are globally unique
// AND immutable, so a project rename cannot orphan a running cluster either.
//
// Readability is preserved by keeping the env id recognizable (env_<hex> →
// env-<hex>); a human maps it back with one lookup, which is the right trade
// against cross-tenant namespace collision.
func (s *Service) resolveNamespace(ctx context.Context, envID string) (string, error) {
	if envID == "" {
		return "", fmt.Errorf("provisioning: cannot resolve a namespace without an environment id")
	}
	return namespaceForEnv(envID), nil
}

// namespaceForEnv maps an environment id to its RFC1123 namespace. Deterministic,
// immutable, and ≤63 chars by construction (env ids are short).
func namespaceForEnv(envID string) string {
	ns := k8sNamespace(envID)
	if len(ns) > 63 {
		ns = strings.Trim(ns[:63], "-")
	}
	return ns
}

var nsInvalid = regexp.MustCompile(`[^a-z0-9-]`)

// k8sNamespace lowercases and dashes a name to an RFC1123 label segment.
func k8sNamespace(name string) string {
	n := nsInvalid.ReplaceAllString(strings.ToLower(name), "-")
	n = strings.Trim(n, "-")
	if n == "" {
		n = "x"
	}
	return n
}

// provisioningSteps is the C4 timeline, born with the row.
func provisioningSteps() []byte {
	steps := []map[string]string{
		{"step": "allocate", "status": "active"},
		{"step": "configure", "status": "pending"},
		{"step": "network/credentials", "status": "pending"},
		{"step": "first backup", "status": "pending"},
		{"step": "ready", "status": "pending"},
	}
	b, _ := json.Marshal(steps)
	return b
}

// CreateServiceInput is the decoded ServiceCreate.
type CreateServiceInput struct {
	Name       string
	Product    string
	Intent     string
	Shape      map[string]any
	EstimateID string
	ActorID    string
}

// CreateService enforces the estimate-before-provision law AT THE API LAYER:
// the estimate must accept (one-shot, env-fenced, live) AND contain a shape
// matching this service — what provisions is what was priced.
func (s *Service) CreateService(ctx context.Context, est *estimates.Service, env store.Environment, orgID string, in CreateServiceInput) (store.Service, error) {
	if in.Name == "" {
		return store.Service{}, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "name", Detail: "required"}})}
	}
	if in.EstimateID == "" {
		return store.Service{}, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "estimate_id", Detail: "required — nothing provisions without an accepted estimate (F2)"}})}
	}
	// Price line for THIS shape (also validates product/size before burning
	// the one-shot estimate).
	line, err := estimates.Price(estimates.ShapeInput{
		Product: in.Product, Intent: in.Intent, Name: in.Name, Shape: in.Shape,
	})
	if err != nil {
		var se estimates.ShapeError
		if errors.As(err, &se) {
			return store.Service{}, problemError{p: problem.ValidationFailed(
				[]problem.FieldError{{Field: se.Field, Detail: se.Detail}})}
		}
		return store.Service{}, err
	}

	// Coverage pre-check BEFORE burning the one-shot estimate (estimate rows
	// are immutable, so this is race-free): a mistyped create keeps the
	// estimate usable — better DX, same law.
	priced, pricedLines, err := est.PricedShapes(ctx, in.EstimateID)
	if err != nil {
		return store.Service{}, err
	}
	// The gate binds to the CONTRACTED CONFIGURATION, not to a price.
	//
	// Matching on (product, price) was substitutable: prices collide — a
	// postgres `dev` with 78 GB and a `standard` both come to 5800¢ — so a
	// caller could price one configuration and provision the other, in either
	// direction, and the gate agreed because the number agreed (US-3.7).
	//
	// estimates.Canonical resolves defaults, so the same configuration spelled
	// differently still matches; what it cannot do is let a DIFFERENT
	// configuration through because it happens to cost the same.
	want, err := estimates.Canonical(estimates.ShapeInput{
		Product: in.Product, Intent: in.Intent, Name: in.Name, Shape: in.Shape,
	})
	if err != nil {
		var se estimates.ShapeError
		if errors.As(err, &se) {
			return store.Service{}, problemError{p: problem.ValidationFailed(
				[]problem.FieldError{{Field: se.Field, Detail: se.Detail}})}
		}
		return store.Service{}, err
	}
	matched := false
	for i, sh := range priced {
		got, cerr := estimates.Canonical(sh)
		if cerr != nil {
			// A stored shape we cannot canonicalize is an internal
			// inconsistency — but estimate rows are IMMUTABLE, so "retry" (the
			// 500 remediation) can never succeed. The actionable answer is the
			// same as any unusable estimate: get a fresh one. The cause is
			// logged rather than surfaced.
			slog.ErrorContext(ctx, "provisioning: stored estimate shape is not canonicalizable",
				"estimate", in.EstimateID, "index", i, "err", cerr)
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"this estimate can no longer be used"},
				"Create a fresh estimate for this environment and accept it — nothing provisions without one.")}
		}
		if got != want {
			continue
		}
		// The price the customer was SHOWN must still be the price they pay.
		// Comparing against a freshly computed price could only ever agree with
		// itself; the stored line is what was on their screen, so a pricing
		// table that moved under a live estimate is caught here.
		//
		// A missing line is an internal inconsistency, never a pass: skipping
		// the check would provision at the CURRENT price with no conflict —
		// silently failing open in a billing guard.
		if i >= len(pricedLines) {
			// Same reasoning as the un-canonicalisable shape above: fail
			// CLOSED, but with remediation the customer can act on. Estimate
			// rows are immutable, so "retry" could never succeed.
			slog.ErrorContext(ctx, "provisioning: estimate has fewer lines than shapes; cannot verify the price shown",
				"estimate", in.EstimateID, "shapes", len(priced), "lines", len(pricedLines))
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"this estimate can no longer be used"},
				"Create a fresh estimate for this environment and accept it — nothing provisions without one.")}
		}
		if pricedLines[i].MonthlyCents != line.MonthlyCents {
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"this shape has been repriced since the estimate was issued"},
				"Create a fresh estimate for this environment and accept it — the price you were shown is the price you pay.")}
		}
		matched = true
		break
	}
	if !matched {
		return store.Service{}, problemError{p: problem.Conflict(
			[]string{"the estimate does not cover this shape"},
			"Estimate the exact shape you are creating, accept it, then create — the estimate IS the contract.")}
	}
	// T11.6 hard spend cap (F9): refuse BEFORE burning the one-shot estimate, so
	// a capped-out create leaves the estimate usable (raise the cap, retry).
	// Enforced here at the API layer — crossing the cap is impossible by
	// construction, never client-only advice.
	if err := s.enforceBudget(ctx, orgID, line.MonthlyCents); err != nil {
		return store.Service{}, err
	}
	if _, _, err := est.Accept(ctx, in.EstimateID, env.ID); err != nil {
		return store.Service{}, err
	}

	// Persist the RESOLVED shape, not the raw request map: what is stored — and
	// what the cell is handed — must be the configuration that was priced and
	// contracted, with defaults explicit rather than implied.
	resolvedShape, err := estimates.Resolve(estimates.ShapeInput{
		Product: in.Product, Intent: in.Intent, Name: in.Name, Shape: in.Shape,
	})
	if err != nil {
		return store.Service{}, err
	}
	shapeJSON, err := json.Marshal(resolvedShape)
	if err != nil {
		return store.Service{}, fmt.Errorf("provisioning: marshal shape: %w", err)
	}
	namespace := namespaceForEnv(env.ID)
	row, err := s.q.InsertService(ctx, store.InsertServiceParams{
		ID: ids.New("svc"), EnvID: env.ID, Name: in.Name, Product: in.Product,
		Intent:               pgtype.Text{String: line.Intent, Valid: true},
		Shape:                shapeJSON,
		ProvisioningSteps:    provisioningSteps(),
		MonthlyEstimateCents: line.MonthlyCents,
		EstimateID:           pgtype.Text{String: in.EstimateID, Valid: true},
		// US-1.3a/US-3.3: desired populated at creation with the resolved cell
		// namespace; the row is outstanding so the cell picks it up next poll.
		Desired: desiredDoc(in.Product, line.Intent, namespace, shapeJSON, nil, nil, false),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"a service with this name already exists in the environment"},
				"Pick a different name.")}
		}
		return store.Service{}, err
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "lifecycle", Via: "user", Actor: in.ActorID,
		Action: "service.created", Subject: row.ID,
		Detail: []byte(`{"name":` + strconv.Quote(in.Name) + `,"product":` + strconv.Quote(in.Product) + `,"estimate":` + strconv.Quote(in.EstimateID) + `}`),
	})
	return row, nil
}

// Transition moves a service along a legal edge, atomically (the SQL guard
// re-checks FROM), records the lifecycle event, and marks step timelines on
// the terminal provisioning edges. Illegal edges are conflicts, not crashes.
func (s *Service) Transition(ctx context.Context, svc store.Service, to, via, actor string, orgID string) (store.Service, error) {
	if !CanTransition(svc.Status, to) {
		return store.Service{}, problemError{p: problem.Conflict(
			[]string{"illegal status transition " + svc.Status + " → " + to},
			"Legal next states from "+svc.Status+": "+fmt.Sprint(transitions[svc.Status])+" (ADR-024).")}
	}
	var steps []byte
	if svc.Status == "provisioning" && to == "ready" {
		b, _ := json.Marshal([]map[string]string{
			{"step": "allocate", "status": "done"},
			{"step": "configure", "status": "done"},
			{"step": "network/credentials", "status": "done"},
			{"step": "first backup", "status": "done"},
			{"step": "ready", "status": "done"},
		})
		steps = b
	}
	row, err := s.q.SetServiceStatus(ctx, store.SetServiceStatusParams{
		ID: svc.ID, Status: svc.Status, Status_2: to, Steps: steps,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"service state changed concurrently"},
				"Re-read the service and retry the transition.")}
		}
		return store.Service{}, err
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "lifecycle", Via: via, Actor: actor,
		Action: "service." + to, Subject: svc.ID,
		Detail: []byte(`{"from":` + strconv.Quote(svc.Status) + `}`),
	})
	// D10: billing span edges — metering starts at ready, never before.
	if edge := metering.BillingEdge(svc.Status, to); edge != "" && s.meter != nil {
		if env, err := s.q.GetEnvironment(ctx, svc.EnvID); err == nil {
			s.meter.MustEmitSpan(ctx, metering.Tags{
				OrgID: orgID, ProjectID: env.ProjectID, EnvID: svc.EnvID, ServiceID: svc.ID,
			}, edge, svc.Product, svc.MonthlyEstimateCents)
		}
	}
	return row, nil
}

// ServiceOrg resolves service → org (404 for unknown ids — no probing).
func (s *Service) ServiceOrg(ctx context.Context, serviceID string) (store.Service, string, error) {
	svc, err := s.q.GetService(ctx, serviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Service{}, "", notFound("service")
	}
	if err != nil {
		return store.Service{}, "", err
	}
	orgID, err := s.q.OrgForService(ctx, serviceID)
	if err != nil {
		return store.Service{}, "", err
	}
	return svc, orgID, nil
}

// UpdateService — shape/scaling are desired-state edits (repriced); the
// manual override requires a reason and auto-expires in 24h (D22).
func (s *Service) UpdateService(ctx context.Context, svc store.Service, orgID, actorID string, shape map[string]any, scaling, override []byte) (store.Service, error) {
	// A deleting service must not be edited: US-1.3a made desired load-bearing,
	// so an edit would rewrite desired with deleting=false and re-outstand the
	// row, cancelling an in-flight teardown. Reject it (before US-1.3a desired
	// was inert '{}', so this clobber is newly reachable).
	if svc.Status == "deleting" {
		return store.Service{}, problemError{p: problem.Conflict(
			[]string{"the service is being deleted"},
			"Wait for deletion to complete; a deleting service cannot be edited.")}
	}
	params := store.UpdateServiceShapeParams{ID: svc.ID}
	if shape != nil {
		// merge over the existing shape: PATCH semantics, absent keys survive
		var current map[string]any
		_ = json.Unmarshal(svc.Shape, &current)
		if current == nil {
			current = map[string]any{}
		}
		for k, v := range shape {
			current[k] = v
		}
		line, err := estimates.Price(estimates.ShapeInput{Product: svc.Product, Name: svc.Name, Shape: current})
		if err != nil {
			var se estimates.ShapeError
			if errors.As(err, &se) {
				return store.Service{}, problemError{p: problem.ValidationFailed(
					[]problem.FieldError{{Field: se.Field, Detail: se.Detail}})}
			}
			return store.Service{}, err
		}
		// Persist the RESOLVED merged shape, exactly as create does. Storing a
		// raw map here and a resolved one there would mean the same
		// configuration is spelled two ways depending on how the service got
		// there — and the identity that gates provisioning is computed from the
		// resolved form.
		resolvedMerged, err := estimates.Resolve(estimates.ShapeInput{Product: svc.Product, Name: svc.Name, Shape: current})
		if err != nil {
			return store.Service{}, err
		}
		merged, err := json.Marshal(resolvedMerged)
		if err != nil {
			return store.Service{}, err
		}
		params.Shape = merged
		params.MonthlyEstimateCents = pgtype.Int8{Int64: line.MonthlyCents, Valid: true}
		// T11.6 hard cap: a scale-UP raises committed monthly spend and MUST
		// respect the cap exactly like a create — otherwise the cap is trivially
		// bypassed by scaling an existing service up. The run-rate already
		// includes this service's OLD cost, so only the increase is projected (a
		// scale-down is always allowed).
		if delta := line.MonthlyCents - svc.MonthlyEstimateCents; delta > 0 {
			if err := s.enforceBudget(ctx, orgID, delta); err != nil {
				return store.Service{}, err
			}
		}
	}
	params.Scaling = scaling
	params.Override = override
	// US-1.3a: rebuild desired from the effective post-edit state and let the
	// query bump generation, so the cell re-reconciles. Effective shape/scaling
	// = the edit if present, else the current row.
	effShape := svc.Shape
	if params.Shape != nil {
		effShape = params.Shape
	}
	effScaling := svc.Scaling
	if scaling != nil {
		effScaling = scaling
	}
	effOverride := svc.Override
	if override != nil {
		effOverride = override
	}
	ns, err := s.resolveNamespace(ctx, svc.EnvID)
	if err != nil {
		return store.Service{}, err
	}
	params.Desired = desiredDoc(svc.Product, svc.Intent.String, ns, effShape, effScaling, effOverride, false)
	row, err := s.q.UpdateServiceShape(ctx, params)
	if err != nil {
		// The SQL fence `status <> 'deleting'` is the atomic backstop for the Go
		// guard above: a delete that raced in after the read returns zero rows.
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"the service is being deleted"},
				"Wait for deletion to complete; a deleting service cannot be edited.")}
		}
		return store.Service{}, err
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "scale", Via: "user", Actor: actorID,
		Action: "service.updated", Subject: svc.ID,
	})
	return row, nil
}

// DeleteService — desired state → deleting (202). The final backup + actual
// teardown are the driver's job (T3.4/US-3.5); dependents (bindings, T3.6)
// join the 409 check when they exist.
func (s *Service) DeleteService(ctx context.Context, svc store.Service, orgID, actorID string) error {
	if svc.Status == "deleting" {
		return problemError{p: problem.Conflict([]string{"deletion already in progress"},
			"The service is already deleting; the final backup will be recorded.")}
	}
	// U6: dependents that will knowingly break are NAMED, all of them.
	deps, err := s.q.ActiveBindingsToTarget(ctx, pgtype.Text{String: svc.ID, Valid: true})
	if err != nil {
		return err
	}
	if len(deps) > 0 {
		reasons := make([]string, 0, len(deps))
		for _, d := range deps {
			reasons = append(reasons, "service "+d.SourceName+" binds to this service ("+d.ID+")")
		}
		return problemError{p: problem.Conflict(reasons,
			"Unbind the listed services first — deleting would knowingly break them (U6).")}
	}
	// US-1.3a: write the deleting desired doc + bump generation (BumpServiceGeneration)
	// so the service becomes outstanding and the cell converges the teardown,
	// THEN transition status to deleting. Two writes, not one transaction (the
	// atomicity hardening is a carried finding). A crash BETWEEN them is not
	// poll-self-healing — the cell would converge the teardown and report `gone`,
	// which is observation-only, so status would stay at its pre-delete value
	// while the desired doc says deleting. It IS retry-recoverable: status is not
	// yet `deleting`, so a second DeleteService passes the guard above and
	// completes the transition. Ordered desired-first deliberately: the reverse
	// (status-first) would strand a row that the guard then refuses to retry.
	dns, err := s.resolveNamespace(ctx, svc.EnvID)
	if err != nil {
		return err
	}
	del := desiredDoc(svc.Product, svc.Intent.String, dns, svc.Shape, svc.Scaling, svc.Override, true)
	if _, err := s.q.BumpServiceGeneration(ctx, store.BumpServiceGenerationParams{ID: svc.ID, Desired: del}); err != nil {
		return err
	}
	_, err = s.Transition(ctx, svc, "deleting", "user", actorID, orgID)
	return err
}

func (s *Service) ListServices(ctx context.Context, envID string) ([]store.Service, error) {
	return s.q.ListServicesForEnv(ctx, envID)
}
