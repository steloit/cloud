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
	"log/slog"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// Actor identifies who performed a change for the audit spine: a user id or a
// token id, with its provenance. Derived from the principal, never the target.
type Actor struct {
	ID  string // usr_ or tok_
	Via string // user | system
}

// keyPattern constrains policy keys to a safe, LIKE-metacharacter-free charset
// (the key is interpolated into the violation-count LIKE pattern).
var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// validateDraft applies the invariants that hold for both create and the
// resulting update: a well-formed key, and the enforce-only-what-we-can-evaluate
// rule — a kind with no registered evaluator may exist as telemetry (warn) but
// must never be a hard block (enforce), which would deny fail-closed.
func (s *Service) validateDraft(key, enforcement string, projectOrg string, orgID string) error {
	if !keyPattern.MatchString(key) {
		return validationError{fields: []problem.FieldError{{Field: "key",
			Detail: "must be lowercase letters, digits and hyphens (e.g. allowed-regions)"}}}
	}
	if enforcement == "enforce" && !s.policyKindKnown(key) {
		return validationError{fields: []problem.FieldError{{Field: "enforcement",
			Detail: fmt.Sprintf("%q has no evaluator yet — keep it in warn (telemetry) until its kind ships; enforce would deny fail-closed", key)}}}
	}
	if projectOrg != "" && projectOrg != orgID {
		return validationError{fields: []problem.FieldError{{Field: "scope.project_id",
			Detail: "project belongs to a different organization"}}}
	}
	return nil
}

// projectOrg returns the owning org of a project id (or "" if none given / not
// found — a missing project is a validation error the caller surfaces).
func (s *Service) projectOrg(ctx context.Context, projectID string) (string, bool, error) {
	if projectID == "" {
		return "", true, nil
	}
	org, err := s.q.GetProjectOrg(ctx, nullText(projectID).String)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return org, true, nil
}

// CreatePolicy persists a new policy (version 1) and records the audit event on
// the spine AFTER the state commits — so a failed insert never orphans an event
// ("the spine never records a create that didn't happen"). A same-scope
// same-key conflict is a 409, caught both pre-check and at the unique constraint
// (the race).
func (s *Service) CreatePolicy(ctx context.Context, d PolicyDraft, actor Actor) (store.Policy, error) {
	porg, ok, err := s.projectOrg(ctx, d.ProjectID)
	if err != nil {
		return store.Policy{}, err
	}
	if !ok {
		return store.Policy{}, validationError{fields: []problem.FieldError{{Field: "scope.project_id", Detail: "project not found"}}}
	}
	if err := s.validateDraft(d.Key, d.Enforcement, porg, d.OrgID); err != nil {
		return store.Policy{}, err
	}
	if n, err := s.q.CountSameKeyPolicies(ctx, store.CountSameKeyPoliciesParams{
		OrgID: d.OrgID, Key: d.Key, ProjectID: nullText(d.ProjectID),
	}); err != nil {
		return store.Policy{}, err
	} else if n > 0 {
		return store.Policy{}, policyConflictError{key: d.Key}
	}

	id := ids.New("pol")
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
	})
	if err != nil {
		return store.Policy{}, mapPolicyWriteErr(err, d.Key)
	}
	if err := q.InsertPolicyVersion(ctx, versionRow(pol, "", actor.ID)); err != nil {
		return store.Policy{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Policy{}, err
	}

	// State is durable: NOW record the audit event and link it back.
	evtID := s.linkPolicyEvent(ctx, pol.OrgID, "policy.created", pol, actor)
	pol.LastChangeEvent = nullText(evtID)
	return pol, nil
}

// UpdatePolicy applies a patch, bumping the version, retaining the prior state
// in policy_versions, and recording the audit event AFTER commit (no orphans).
func (s *Service) UpdatePolicy(ctx context.Context, id string, patch PolicyPatch, actor Actor) (store.Policy, error) {
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
	// same enforce-only-what-we-can-evaluate gate as create (promote-to-enforce).
	if enforcement == "enforce" && !s.policyKindKnown(cur.Key) {
		return store.Policy{}, validationError{fields: []problem.FieldError{{Field: "enforcement",
			Detail: fmt.Sprintf("%q has no evaluator yet — keep it in warn until its kind ships", cur.Key)}}}
	}
	config := cur.Config
	if patch.Config != nil {
		config = jsonOrEmpty(patch.Config)
	}
	enforceFrom := cur.EnforceFrom
	if patch.EnforceFrom != nil {
		enforceFrom = nullTime(patch.EnforceFrom)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return store.Policy{}, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	updated, err := q.UpdatePolicyRow(ctx, store.UpdatePolicyRowParams{
		ID: id, Enforcement: enforcement, Rule: cur.Rule, Config: config,
		Description: cur.Description, EnforceFrom: enforceFrom,
	})
	if err != nil {
		return store.Policy{}, err
	}
	if err := q.InsertPolicyVersion(ctx, versionRow(updated, "", actor.ID)); err != nil {
		return store.Policy{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Policy{}, err
	}

	evtID := s.linkPolicyEvent(ctx, updated.OrgID, "policy.updated", updated, actor)
	updated.LastChangeEvent = nullText(evtID)
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
		OrgID: orgID, Key: nullText(key),
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

func policyDetail(key, enforcement string, version int32) []byte {
	return []byte(fmt.Sprintf(`{"key":%q,"enforcement":%q,"version":%d}`, key, enforcement, version))
}

func jsonOrEmpty(b []byte) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

// linkPolicyEvent appends the audit event AFTER the state is committed and links
// its id back onto the row + version (G7). A ledger failure here is logged
// loudly, never rolled back onto the committed change (the codebase's outbox
// discipline until the tx-outbox lands). Returns the event id ("" on failure).
func (s *Service) linkPolicyEvent(ctx context.Context, orgID, action string, p store.Policy, actor Actor) string {
	if s.rec == nil {
		return ""
	}
	evt, err := s.rec.Append(ctx, events.Input{
		OrgID: orgID, Kind: "policy", Via: actor.Via, Actor: actor.ID,
		Action: action, Subject: p.ID, Detail: policyDetail(p.Key, p.Enforcement, p.Version),
	})
	if err != nil {
		slog.Error("events: policy audit append failed after commit", "action", action, "policy", p.ID, "err", err)
		return ""
	}
	if err := s.q.LinkPolicyEvent(ctx, store.LinkPolicyEventParams{ID: p.ID, LastChangeEvent: nullText(evt.ID)}); err != nil {
		slog.Error("events: policy event link failed", "policy", p.ID, "err", err)
	}
	if err := s.q.LinkPolicyVersionEvent(ctx, store.LinkPolicyVersionEventParams{PolicyID: p.ID, Version: p.Version, ChangeEvent: nullText(evt.ID)}); err != nil {
		slog.Error("events: policy version event link failed", "policy", p.ID, "err", err)
	}
	return evt.ID
}

// mapPolicyWriteErr turns the unique/foreign-key violations of the policies
// insert into the right client errors: a same-scope duplicate (the conflict
// pre-check's race) is a 409, a bad project_id an FK violation → 422.
func mapPolicyWriteErr(err error, key string) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505": // unique_violation
			return policyConflictError{key: key}
		case "23503": // foreign_key_violation (project_id)
			return validationError{fields: []problem.FieldError{{Field: "scope.project_id", Detail: "project not found"}}}
		}
	}
	return err
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
