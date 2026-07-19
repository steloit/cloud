package identity

// T12.1: the policy AUTHORING service over the T2.4 evaluation substrate.
// CRUD + versioning + dry-run impact preview. Policies are a governed resource
// (ADR-0006): every mutation routes through the Authorizer (policy.manage) at
// the handler, and emits a lifecycle event to the append-only spine whose id is
// stamped back onto the row as last_change_event (G7). Every version is retained
// in policy_versions so a promote/revert is audited, never reconstructed.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

var ErrPolicyNotFound = errors.New("identity: policy not found")

// aiAssistantKey is the one key whose enforcement vocabulary is the
// enabled|opt_in|disabled triad; every other (rule) key is warn|enforce.
const aiAssistantKey = "ai-assistant"

// PolicyDraft is a create/preview request in storage-neutral terms.
type PolicyDraft struct {
	OrgID       string
	Key         string
	ProjectID   string // "" = org-wide
	Description string
	Enforcement string // validated per key
	Rule        []byte // jsonb (the structured rule; may be "{}")
	EnforceFrom *time.Time
}

// PolicyPatch is a partial update; nil fields are unchanged.
type PolicyPatch struct {
	Enforcement *string
	Config      []byte // nil = unchanged
	EnforceFrom *time.Time
}

// PolicyImpact is the dry-run preview — the governance analog of
// estimate-before-provision (G11): what a policy would touch, before enforce.
type PolicyImpact struct {
	Conflicts       []PolicyConflict
	MembersAffected int
	Affected        []AffectedResource
}

type PolicyConflict struct {
	WithPolicy  string
	Detail      string
	Resolutions []string
}

type AffectedResource struct {
	ID     string
	Name   string
	Effect string
}

// ValidatePolicyEnforcement enforces the per-key vocabulary. Empty defaults to
// warn (warn-first posture) for rule keys, opt_in for ai-assistant.
func ValidatePolicyEnforcement(key, enforcement string) (string, error) {
	if key == aiAssistantKey {
		if enforcement == "" {
			return "opt_in", nil
		}
		switch enforcement {
		case "enabled", "opt_in", "disabled":
			return enforcement, nil
		}
		return "", fmt.Errorf("ai-assistant policy uses enabled|opt_in|disabled, not %q", enforcement)
	}
	if enforcement == "" {
		return "warn", nil // warn-first default posture (G9)
	}
	switch enforcement {
	case "warn", "enforce":
		return enforcement, nil
	}
	return "", fmt.Errorf("rule policy uses warn|enforce (warn-first), not %q", enforcement)
}

func nullText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func nullTime(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// PreviewPolicy computes the dry-run impact WITHOUT persisting anything.
func (s *Service) PreviewPolicy(ctx context.Context, d PolicyDraft) (PolicyImpact, error) {
	same, err := s.q.ListSameKeyPolicies(ctx, store.ListSameKeyPoliciesParams{
		OrgID: d.OrgID, Key: d.Key, ProjectID: nullText(d.ProjectID),
	})
	if err != nil {
		return PolicyImpact{}, err
	}
	members, err := s.q.CountOrgMembers(ctx, d.OrgID)
	if err != nil {
		return PolicyImpact{}, err
	}
	impact := PolicyImpact{MembersAffected: int(members)}
	for _, p := range same {
		scope := "org-wide"
		if p.ProjectID.Valid {
			scope = "project " + p.ProjectID.String
		}
		impact.Conflicts = append(impact.Conflicts, PolicyConflict{
			WithPolicy: p.ID,
			Detail:     fmt.Sprintf("an existing %q policy (%s) already governs this scope", p.Key, scope),
			Resolutions: []string{
				"edit the existing policy instead of creating a second",
				"attach this one to a narrower project scope (closest-wins overrides org-wide)",
			},
		})
	}
	return impact, nil
}

// CreatePolicy persists a new policy (version 1), records the audit event, and
// stamps its id back as last_change_event. Conflict (a same-scope same-key
// policy) is refused BEFORE any write, so the spine never records a create that
// didn't happen.
func (s *Service) CreatePolicy(ctx context.Context, d PolicyDraft, actor string) (store.Policy, error) {
	n, err := s.q.CountSameKeyPolicies(ctx, store.CountSameKeyPoliciesParams{
		OrgID: d.OrgID, Key: d.Key, ProjectID: nullText(d.ProjectID),
	})
	if err != nil {
		return store.Policy{}, err
	}
	if n > 0 {
		return store.Policy{}, policyConflictError{key: d.Key}
	}

	id := ids.New("pol")
	evt, err := s.rec.Append(ctx, events.Input{
		OrgID: d.OrgID, Kind: "policy", Via: viaFor(actor), Actor: actor,
		Action: "policy.created", Subject: id,
		Detail: policyDetail(d.Key, d.Enforcement, 1),
	})
	if err != nil {
		return store.Policy{}, fmt.Errorf("identity: policy audit: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return store.Policy{}, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	pol, err := q.InsertPolicy(ctx, store.InsertPolicyParams{
		ID: id, OrgID: d.OrgID, ProjectID: nullText(d.ProjectID), Key: d.Key,
		Enforcement: d.Enforcement, Rule: jsonOrEmpty(d.Rule), Config: []byte("{}"),
		Description: nullText(d.Description), EnforceFrom: nullTime(d.EnforceFrom),
		LastChangeEvent: nullText(evt.ID),
	})
	if err != nil {
		return store.Policy{}, err
	}
	if err := q.InsertPolicyVersion(ctx, versionRow(pol, evt.ID, actor)); err != nil {
		return store.Policy{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Policy{}, err
	}
	return pol, nil
}

// UpdatePolicy applies a patch, bumping the version, retaining the prior state
// in policy_versions, and linking the audit event.
func (s *Service) UpdatePolicy(ctx context.Context, id string, patch PolicyPatch, actor string) (store.Policy, error) {
	cur, err := s.q.GetPolicyByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Policy{}, ErrPolicyNotFound
		}
		return store.Policy{}, err
	}

	enforcement := cur.Enforcement
	if patch.Enforcement != nil {
		enforcement, err = ValidatePolicyEnforcement(cur.Key, *patch.Enforcement)
		if err != nil {
			return store.Policy{}, validationError{fields: []problem.FieldError{{Field: "enforcement", Detail: err.Error()}}}
		}
	}
	config := cur.Config
	if patch.Config != nil {
		config = jsonOrEmpty(patch.Config)
	}
	enforceFrom := cur.EnforceFrom
	if patch.EnforceFrom != nil {
		enforceFrom = nullTime(patch.EnforceFrom)
	}

	evt, err := s.rec.Append(ctx, events.Input{
		OrgID: cur.OrgID, Kind: "policy", Via: viaFor(actor), Actor: actor,
		Action: "policy.updated", Subject: id,
		Detail: policyDetail(cur.Key, enforcement, int(cur.Version)+1),
	})
	if err != nil {
		return store.Policy{}, fmt.Errorf("identity: policy audit: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return store.Policy{}, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	updated, err := q.UpdatePolicyRow(ctx, store.UpdatePolicyRowParams{
		ID: id, Enforcement: enforcement, Rule: cur.Rule, Config: config,
		Description: cur.Description, EnforceFrom: enforceFrom, LastChangeEvent: nullText(evt.ID),
	})
	if err != nil {
		return store.Policy{}, err
	}
	if err := q.InsertPolicyVersion(ctx, versionRow(updated, evt.ID, actor)); err != nil {
		return store.Policy{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Policy{}, err
	}
	return updated, nil
}

func (s *Service) GetPolicy(ctx context.Context, id string) (store.Policy, error) {
	pol, err := s.q.GetPolicyByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Policy{}, ErrPolicyNotFound
		}
		return store.Policy{}, err
	}
	return pol, nil
}

func (s *Service) ListPolicies(ctx context.Context, orgID string) ([]store.Policy, error) {
	return s.q.ListPoliciesPage(ctx, orgID)
}

func (s *Service) PolicyVersions(ctx context.Context, id string) ([]store.PolicyVersion, error) {
	return s.q.ListPolicyVersions(ctx, id)
}

// PolicyViolations30d is the warn-mode telemetry (G12): denials attributed to
// this policy key on the spine over the trailing 30 days.
func (s *Service) PolicyViolations30d(ctx context.Context, orgID, key string) (int, error) {
	n, err := s.q.CountPolicyViolations30d(ctx, store.CountPolicyViolations30dParams{
		OrgID: orgID, Column2: nullText(key),
	})
	return int(n), err
}

func versionRow(p store.Policy, eventID, actor string) store.InsertPolicyVersionParams {
	return store.InsertPolicyVersionParams{
		ID: ids.New("pv"), PolicyID: p.ID, Version: p.Version, Key: p.Key,
		Enforcement: p.Enforcement, Rule: p.Rule, Config: p.Config,
		Description: p.Description, EnforceFrom: p.EnforceFrom,
		ChangeEvent: nullText(eventID), ChangedBy: actor,
	}
}

func policyDetail(key, enforcement string, version int) []byte {
	return []byte(fmt.Sprintf(`{"key":%q,"enforcement":%q,"version":%d}`, key, enforcement, version))
}

func jsonOrEmpty(b []byte) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

// viaFor tags the spine's provenance column: a token actor is a machine.
func viaFor(actor string) string {
	if len(actor) >= 4 && actor[:4] == "tok_" {
		return "system"
	}
	return "user"
}

// policyConflictError renders the 409 as a checklist with fixes (G11), never a
// wall of red.
type policyConflictError struct{ key string }

func (e policyConflictError) Error() string { return "identity: policy conflict on " + e.key }

func (e policyConflictError) Problem() problem.Problem {
	return problem.Conflict(
		[]string{fmt.Sprintf("a %q policy already governs this scope", e.key)},
		"Edit the existing policy, or attach this one to a narrower project scope (closest-wins overrides org-wide).",
	)
}
