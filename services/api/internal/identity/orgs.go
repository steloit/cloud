package identity

// T2.7: org governance — creation (org + inert subscription + owner, one tx),
// slug immutability, member management with the ≥1-owner guard, G6 removal
// semantics, and seat accounting (Free 3 / Pro 5 / Business 20, $7/seat).

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

var (
	ErrSlugTaken   = errors.New("identity: organization slug already taken")
	ErrLastOwner   = errors.New("identity: an organization keeps at least one owner")
	ErrOrgDeleting = errors.New("identity: deletion already scheduled")
)

var slugRe = regexp.MustCompile(`^[a-z0-9-]{3,32}$`)

// Slugify derives the immutable slug from the org name (contract: slugified
// [a-z0-9-]{3,32}; immutable after create).
func Slugify(name string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(name))
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if !slugRe.MatchString(s) {
		return "", fmt.Errorf("name must slugify to [a-z0-9-]{3,32}, got %q", s)
	}
	return s, nil
}

// SeatAllowance is BR canon (feature-specs F1): seats per plan, then $7/seat
// prorated. Data, not policy — the numbers live in one place.
// CreateOrgWithOwner creates org + inert subscription row + owner membership
// in ONE transaction (closing T2.3's recorded promise), then records the
// spine events after commit.
func (s *Service) CreateOrgWithOwner(ctx context.Context, name, ownerUserID string) (store.Org, error) {
	return s.CreateOrgFull(ctx, name, "aws/ap-south-1", ownerUserID)
}

func (s *Service) CreateOrgFull(ctx context.Context, name, homeRegion, ownerUserID string) (store.Org, error) {
	slug, err := Slugify(name)
	if err != nil {
		return store.Org{}, validationError{fields: []problem.FieldError{{Field: "name", Detail: err.Error()}}}
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return store.Org{}, fmt.Errorf("identity: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	org, err := q.CreateOrgFull(ctx, store.CreateOrgFullParams{
		ID: ids.New("org"), Slug: slug, Name: name, HomeRegion: homeRegion,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return store.Org{}, ErrSlugTaken
		}
		return store.Org{}, fmt.Errorf("identity: create org: %w", err)
	}
	if _, err := q.CreateSubscription(ctx, org.ID); err != nil {
		return store.Org{}, fmt.Errorf("identity: subscription row: %w", err)
	}
	if _, err := q.AddMember(ctx, store.AddMemberParams{
		ID: ids.New("mbr"), OrgID: org.ID, UserID: ownerUserID, Role: "owner",
	}); err != nil {
		return store.Org{}, fmt.Errorf("identity: owner membership: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Org{}, fmt.Errorf("identity: commit: %w", err)
	}
	s.record(ctx, events.Input{
		OrgID: org.ID, Kind: "lifecycle", Via: "user", Actor: ownerUserID,
		Action: "org.created", Subject: org.ID,
		Detail: []byte(`{"name":` + strconv.Quote(name) + `,"slug":` + strconv.Quote(slug) + `}`),
	})
	s.record(ctx, events.Input{
		OrgID: org.ID, Kind: "membership", Via: "user", Actor: ownerUserID,
		Action: "member.added", Subject: ownerUserID, Detail: []byte(`{"role":"owner"}`),
	})
	return org, nil
}

func (s *Service) GetOrg(ctx context.Context, orgID string) (store.Org, error) {
	org, err := s.q.GetOrg(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Org{}, notFoundError{what: "organization"}
	}
	return org, err
}

func (s *Service) ListOrgsForUser(ctx context.Context, userID string) ([]store.Org, error) {
	return s.q.ListOrgsForUser(ctx, userID)
}

// UpdateOrg renames / changes the region prefill. The slug NEVER changes —
// no code path writes it after create.
func (s *Service) UpdateOrg(ctx context.Context, orgID, actorID string, name, homeRegion *string) (store.Org, error) {
	params := store.UpdateOrgParams{ID: orgID}
	if name != nil {
		params.Name = txt(*name)
	}
	if homeRegion != nil {
		params.HomeRegion = txt(*homeRegion)
	}
	org, err := s.q.UpdateOrg(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Org{}, notFoundError{what: "organization"}
	}
	if err != nil {
		return store.Org{}, err
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "lifecycle", Via: "user", Actor: actorID,
		Action: "org.updated", Subject: orgID,
	})
	return org, nil
}

// ScheduleOrgDeletion — 202 semantics: state kept per the dunning/deletion
// contract (90-day billing-data rule); actual purge is a later job.
func (s *Service) ScheduleOrgDeletion(ctx context.Context, orgID, actorID string) error {
	n, err := s.q.ScheduleOrgDeletion(ctx, orgID)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrOrgDeleting
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "lifecycle", Via: "user", Actor: actorID,
		Action: "org.deletion_scheduled", Subject: orgID,
	})
	return nil
}

// SeatInfo mirrors MemberList.seats.
type SeatInfo struct {
	Included, Used    int
	OveragePriceCents int
}

func (s *Service) Seats(ctx context.Context, org store.Org) (SeatInfo, error) {
	used, err := s.q.CountMembers(ctx, org.ID)
	if err != nil {
		return SeatInfo{}, err
	}
	return SeatInfo{
		Included:          s.plans.IncludedSeats(org.Plan),
		Used:              int(used),
		OveragePriceCents: s.plans.Overage.SeatCents,
	}, nil
}

func (s *Service) ListMembers(ctx context.Context, orgID string) ([]store.ListMembersRow, error) {
	return s.q.ListMembers(ctx, orgID)
}

// ChangeMemberRole applies without re-login (authorization is evaluated per
// request from the membership row) and audits before → after. The last owner
// cannot be demoted — DB trigger, mapped to 409.
func (s *Service) ChangeMemberRole(ctx context.Context, orgID, memberID, role, actorID string) (store.Member, error) {
	before, err := s.q.GetMember(ctx, store.GetMemberParams{OrgID: orgID, ID: memberID})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Member{}, notFoundError{what: "member"}
	}
	if err != nil {
		return store.Member{}, err
	}
	m, err := s.q.UpdateMemberRole(ctx, store.UpdateMemberRoleParams{OrgID: orgID, ID: memberID, Role: role})
	if err != nil {
		if isLastOwner(err) {
			return store.Member{}, ErrLastOwner
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Member{}, notFoundError{what: "member"}
		}
		return store.Member{}, err
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "membership", Via: "user", Actor: actorID,
		Action: "member.role_changed", Subject: m.UserID,
		Detail: []byte(`{"before":` + strconv.Quote(before.Role) + `,"after":` + strconv.Quote(role) + `}`),
	})
	return m, nil
}

// FlaggedResource is a removed member's owned resource — flagged, never
// silently reassigned (G6). Nothing ownable exists yet (dashboards arrive in
// E10); the contract shape ships now so removal is honest from day one.
type FlaggedResource struct {
	ID, Kind, Name string
}

// RemoveMember: G6 removal semantics — the membership row dies, and the
// user's sessions + personal tokens are revoked immediately ("every personal
// token dies with the membership", G8). The last owner cannot be removed.
func (s *Service) RemoveMember(ctx context.Context, orgID, memberID, actorID string) ([]FlaggedResource, error) {
	m, err := s.q.RemoveMember(ctx, store.RemoveMemberParams{OrgID: orgID, ID: memberID})
	if err != nil {
		if isLastOwner(err) {
			return nil, ErrLastOwner
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, notFoundError{what: "member"}
		}
		return nil, err
	}
	if err := s.q.RevokeAllSessionsForUser(ctx, m.UserID); err != nil {
		return nil, fmt.Errorf("identity: revoke sessions: %w", err)
	}
	if err := s.q.RevokeAllPersonalTokensForUser(ctx, txt(m.UserID)); err != nil {
		return nil, fmt.Errorf("identity: revoke tokens: %w", err)
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "membership", Via: "user", Actor: actorID,
		Action: "member.removed", Subject: m.UserID,
		Detail: []byte(`{"role":` + strconv.Quote(m.Role) + `}`),
	})
	return []FlaggedResource{}, nil
}

func isLastOwner(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514" && strings.Contains(pgErr.Message, "last owner")
}

// ScheduleAccountDeletion schedules the caller's account for deletion in ONE
// transaction: the sole-owner gate, the schedule write, and the immediate
// revocation of every session + personal token all commit together (or not
// at all). Returns the orgs the user remains a member of (for spine events)
// and the sole-owned blockers (empty when it proceeds).
func (s *Service) ScheduleAccountDeletion(ctx context.Context, userID string) (orgs []string, blockers []store.OwnedSoleOwnerOrgsRow, err error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("identity: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	sole, err := q.OwnedSoleOwnerOrgs(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if len(sole) > 0 {
		return nil, sole, nil // caller maps to 409; nothing written
	}
	n, err := q.ScheduleAccountDeletion(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if n == 0 {
		return nil, nil, ErrOrgDeleting // reused sentinel: already scheduled
	}
	// Revocation is part of the SAME tx — if either fails, the schedule rolls
	// back, so a retry re-runs cleanly (no "scheduled but not revoked" state).
	if err := q.RevokeAllSessionsForUser(ctx, userID); err != nil {
		return nil, nil, err
	}
	if err := q.RevokeAllPersonalTokensForUser(ctx, txt(userID)); err != nil {
		return nil, nil, err
	}
	memberOrgs, err := q.OrgsForMember(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("identity: commit: %w", err)
	}
	return memberOrgs, nil, nil
}

// ListInvoices reads an org's invoices (T11.3; billing.view). The generator
// (internal/invoice) writes them; this is the B3 read surface.
func (s *Service) ListInvoices(ctx context.Context, orgID string) ([]store.Invoice, error) {
	return s.q.ListInvoices(ctx, orgID)
}
