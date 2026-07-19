package identity

// MailDirectory adapts the identity store into mailer.Directory (T10.4): it
// resolves the recipient + template data an email needs from a spine event's
// subject, keeping the mailer package free of identity domain knowledge.

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/steloit/cloud/services/api/internal/identity/store"
)

type MailDirectory struct {
	q           *store.Queries
	consoleBase string
}

func NewMailDirectory(q *store.Queries, consoleBase string) *MailDirectory {
	return &MailDirectory{q: q, consoleBase: consoleBase}
}

// InviteEmail resolves an invite.created event (Subject = invite id) to the
// invitee's address, its org, and the template data (org name, role, the accept
// link). A vanished invite (revoked/expired between event and send) yields an
// empty recipient so the dispatcher skips it, never errors the batch.
func (m *MailDirectory) InviteEmail(ctx context.Context, inviteID string) (recipient, orgID string, data map[string]string, err error) {
	inv, err := m.q.GetInvite(ctx, inviteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", nil, nil
		}
		return "", "", nil, err
	}
	org, err := m.q.GetOrg(ctx, inv.OrgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", nil, nil // org gone → skip, never a poison-pill error
		}
		return "", "", nil, err
	}
	data = map[string]string{
		"org":        org.Name,
		"role":       inv.Role,
		"accept_url": m.consoleBase + "/invite/" + inviteID,
	}
	return inv.Email, inv.OrgID, data, nil
}
