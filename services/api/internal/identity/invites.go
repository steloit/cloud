package identity

// T2.7: invite lifecycle (QA scenario 8) — 7-day expiry (computed, never
// cron-flipped), pending dedupe, wrong-account acceptance blocked, seat
// overage 402 with explicit confirm=true, notifications as spine events.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/quota"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

const inviteTTLDays = 7

var (
	ErrAlreadyMember    = errors.New("identity: already a member")
	ErrAlreadyInvited   = errors.New("identity: already invited")
	ErrWrongAccount     = errors.New("identity: invite bound to another address")
	ErrInviteGone       = errors.New("identity: invite expired, used or revoked")
	ErrInviteNotExpired = errors.New("identity: invite is not expired")
)

// SeatOverageError carries the 402 math (x-error-catalog quota_exceeded).
type SeatOverageError struct{ PriceCents int64 }

func (e SeatOverageError) Error() string { return "identity: seat limit — confirm the overage" }

// newInviteID mints the capability id: the id IS the public link, so it
// carries real entropy, not a short display id.
func newInviteID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("identity: invite entropy: %w", err)
	}
	return "inv_" + hex.EncodeToString(b), nil
}

// InviteStatus computes the effective status: a pending row past expires_at
// IS expired — no cron flips rows.
func InviteStatus(inv store.Invite) string {
	if inv.Status == "pending" && inv.ExpiresAt.Valid && inv.ExpiresAt.Time.Before(time.Now()) {
		return "expired"
	}
	return inv.Status
}

// CreateInvite — dedupe against members + pending; seats: soft 402 unless
// confirm (the price is printed, never discovered).
func (s *Service) CreateInvite(ctx context.Context, orgID, email, role, inviterID string, confirm bool) (store.Invite, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if err := ValidateEmail(email); err != nil {
		return store.Invite{}, validationError{fields: []problem.FieldError{{Field: "email", Detail: err.Error()}}}
	}
	if isMember, err := s.q.IsMemberEmail(ctx, store.IsMemberEmailParams{OrgID: orgID, Lower: email}); err != nil {
		return store.Invite{}, err
	} else if isMember {
		return store.Invite{}, ErrAlreadyMember
	}
	if pending, err := s.q.GetPendingInviteByEmail(ctx, store.GetPendingInviteByEmailParams{OrgID: orgID, Lower: email}); err == nil {
		if InviteStatus(pending) == "pending" {
			return store.Invite{}, ErrAlreadyInvited
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return store.Invite{}, err
	}

	org, err := s.GetOrg(ctx, orgID)
	if err != nil {
		return store.Invite{}, err
	}
	seats, err := s.Seats(ctx, org)
	if err != nil {
		return store.Invite{}, err
	}
	// Seats are a SOFT quota: adding a member over the plan allowance proceeds
	// only with confirm=true and the price shown — the T11.5 evaluator decides,
	// so soft-quota behaviour is identical to every other metered limit.
	if d := quota.Evaluate(quota.Soft, int64(seats.Included), int64(seats.Used), 1,
		int64(seats.OveragePriceCents), confirm); d.SoftBlocked {
		return store.Invite{}, SeatOverageError{PriceCents: d.OveragePriceCents}
	}

	id, err := newInviteID()
	if err != nil {
		return store.Invite{}, err
	}
	inv, err := s.q.CreateInvite(ctx, store.CreateInviteParams{
		ID: id, OrgID: orgID, Email: email, Role: role, InviterID: inviterID,
		ExpiresAt: tstz(s.now().AddDate(0, 0, inviteTTLDays)),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return store.Invite{}, ErrAlreadyInvited // raced the partial-unique index
		}
		return store.Invite{}, err
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "membership", Via: "user", Actor: inviterID,
		Action: "invite.created", Subject: inv.ID,
		Detail: []byte(`{"role":` + strconv.Quote(role) + `,"confirmed_overage":` + strconv.FormatBool(seats.Used >= seats.Included) + `}`),
	})
	return inv, nil
}

func (s *Service) ListInvites(ctx context.Context, orgID string) ([]store.Invite, error) {
	return s.q.ListInvites(ctx, orgID)
}

// RevokeInvite (G-side): invalidates the link, notifies nobody, audited.
func (s *Service) RevokeInvite(ctx context.Context, orgID, inviteID, actorID string) error {
	inv, err := s.q.GetInvite(ctx, inviteID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && inv.OrgID != orgID) {
		return notFoundError{what: "invite"}
	}
	if err != nil {
		return err
	}
	n, err := s.q.SetInviteStatus(ctx, store.SetInviteStatusParams{ID: inviteID, Status: "revoked"})
	if err != nil {
		return err
	}
	if n == 0 {
		return notFoundError{what: "pending invite"}
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "membership", Via: "user", Actor: actorID,
		Action: "invite.revoked", Subject: inviteID,
	})
	return nil
}

// PublicInvite is the unauthenticated A6 view: role explained by
// consequences, email masked, status honest.
type PublicInvite struct {
	InviterName, OrgName, Role, EmailHint, Status string
	RoleConsequences                              []string
}

// roleConsequences follows the A6 grammar ("what accepting does");
// Developer's line is the frame's copy. Other roles are matrix-faithful
// derivations pending their own frame pass (S9 candidate, noted in the PR).
var roleConsequences = map[string][]string{
	"owner": {
		"Full control: members, billing, policies and org settings — including deleting the org.",
		"Roles are set by the org — yours can change later without a new invite.",
	},
	"admin": {
		"Manage members, projects, services and policies · can't manage billing or delete the org.",
		"Roles are set by the org — yours can change later without a new invite.",
	},
	"developer": {
		"Create and deploy services, read metrics and logs · can't manage members, billing or org policies.",
		"Roles are set by the org — yours can change later without a new invite.",
	},
	"billing": {
		"View and manage billing, plans and payment methods · can't create or change services.",
		"Roles are set by the org — yours can change later without a new invite.",
	},
}

func maskEmail(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 1 {
		return "***" + email[max(at, 0):]
	}
	return email[:1] + "***" + email[at:]
}

// PublicInviteView — no auth; expired/used/revoked callers get ErrInviteGone
// (410 distinguishing the A7 states via Status).
func (s *Service) PublicInviteView(ctx context.Context, inviteID string) (PublicInvite, error) {
	inv, err := s.q.GetInvite(ctx, inviteID)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicInvite{}, notFoundError{what: "invite"}
	}
	if err != nil {
		return PublicInvite{}, err
	}
	org, err := s.q.GetOrg(ctx, inv.OrgID)
	if err != nil {
		return PublicInvite{}, err
	}
	inviter, err := s.q.GetUserByID(ctx, inv.InviterID)
	if err != nil {
		return PublicInvite{}, err
	}
	out := PublicInvite{
		InviterName: inviter.Name, OrgName: org.Name, Role: inv.Role,
		EmailHint: maskEmail(inv.Email), Status: InviteStatus(inv),
		RoleConsequences: roleConsequences[inv.Role],
	}
	if out.Status != "pending" {
		return out, ErrInviteGone
	}
	return out, nil
}

// AcceptInvite — the session email must match the invited address (A7 card
// 3): wrong account is 403, never a silent join.
func (s *Service) AcceptInvite(ctx context.Context, inviteID, userID string) error {
	inv, err := s.q.GetInvite(ctx, inviteID)
	if errors.Is(err, pgx.ErrNoRows) {
		return notFoundError{what: "invite"}
	}
	if err != nil {
		return err
	}
	if InviteStatus(inv) != "pending" {
		return ErrInviteGone
	}
	u, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if !strings.EqualFold(u.Email, inv.Email) {
		return ErrWrongAccount
	}
	if _, err := s.q.AddMember(ctx, store.AddMemberParams{
		ID: ids.New("mbr"), OrgID: inv.OrgID, UserID: userID, Role: inv.Role,
	}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Already a member (raced or re-click): the invite is used up,
			// membership stands — QA 8's "already-used → sign in" path.
			_, _ = s.q.SetInviteStatus(ctx, store.SetInviteStatusParams{ID: inviteID, Status: "accepted"})
			return nil
		}
		return err
	}
	if _, err := s.q.SetInviteStatus(ctx, store.SetInviteStatusParams{ID: inviteID, Status: "accepted"}); err != nil {
		return err
	}
	s.record(ctx, events.Input{
		OrgID: inv.OrgID, Kind: "membership", Via: "user", Actor: userID,
		Action: "member.added", Subject: userID,
		Detail: []byte(`{"role":` + strconv.Quote(inv.Role) + `,"invite":` + strconv.Quote(inviteID) + `}`),
	})
	return nil
}

// DeclineInvite — notifies the inviter (a spine event; email routing rides
// the spine when 23-emails lands) and invalidates the link.
func (s *Service) DeclineInvite(ctx context.Context, inviteID string) error {
	inv, err := s.q.GetInvite(ctx, inviteID)
	if errors.Is(err, pgx.ErrNoRows) {
		return notFoundError{what: "invite"}
	}
	if err != nil {
		return err
	}
	if InviteStatus(inv) != "pending" {
		return ErrInviteGone
	}
	if _, err := s.q.SetInviteStatus(ctx, store.SetInviteStatusParams{ID: inviteID, Status: "declined"}); err != nil {
		return err
	}
	s.record(ctx, events.Input{
		OrgID: inv.OrgID, Kind: "membership", Via: "system", Actor: "system",
		Action: "invite.declined", Subject: inviteID,
	})
	return nil
}

// RenewInvite — A7's "one click fixes it": only meaningful for an EXPIRED
// invite; the inviter is notified via the spine (bell/email routing later).
func (s *Service) RenewInvite(ctx context.Context, inviteID, rateKey string) error {
	// A leaked (expired) invite id must not be loopable to flood the inviter:
	// this endpoint is public and stateless (renewal is a request, not a state
	// change), so rate-limit by requester before doing any work.
	if ok, retry := s.limiter.Allow("invrenew|" + rateKey); !ok {
		return RateLimitedError{RetryAfterS: retry}
	}
	inv, err := s.q.GetInvite(ctx, inviteID)
	if errors.Is(err, pgx.ErrNoRows) {
		return notFoundError{what: "invite"}
	}
	if err != nil {
		return err
	}
	if InviteStatus(inv) != "expired" {
		return ErrInviteNotExpired
	}
	s.record(ctx, events.Input{
		OrgID: inv.OrgID, Kind: "membership", Via: "system", Actor: "system",
		Action: "invite.renewal_requested", Subject: inviteID,
		Detail: []byte(`{"inviter":` + strconv.Quote(inv.InviterID) + `}`),
	})
	return nil
}
