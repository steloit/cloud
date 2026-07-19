package identity

// MailDirectory adapts the identity store into mailer.Directory (T10.4): it
// resolves the recipient + template data an email needs from a spine event's
// subject, keeping the mailer package free of identity domain knowledge.

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/mailer"
	"github.com/steloit/cloud/services/api/internal/secrets"
)

type MailDirectory struct {
	q           *store.Queries
	consoleBase string
	kek         secrets.KEK // to unseal reset tokens for the reset link
}

func NewMailDirectory(q *store.Queries, consoleBase string, kek secrets.KEK) *MailDirectory {
	return &MailDirectory{q: q, consoleBase: consoleBase, kek: kek}
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

// PasswordResetEmail resolves a password.reset_requested event (Subject = reset
// token id) to the account's address and the reset link, unsealing the token's
// plaintext from the row (KEK-protected, AAD-bound). A vanished token yields an
// empty recipient so the dispatcher skips it.
func (m *MailDirectory) PasswordResetEmail(ctx context.Context, resetID string) (recipient, orgID string, data map[string]string, err error) {
	row, err := m.q.GetResetToken(ctx, resetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", nil, nil
		}
		return "", "", nil, err
	}
	u, err := m.q.GetUserByID(ctx, row.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", nil, nil
		}
		return "", "", nil, err
	}
	token, err := secrets.Open(m.kek, secrets.Sealed{
		Ciphertext: row.Ciphertext, Nonce: row.Nonce,
		WrappedDEK: row.WrappedDek, DEKNonce: row.DekNonce, KEKID: row.KekID,
	}, []byte("pwreset:"+resetID))
	if err != nil {
		return "", "", nil, err
	}
	data = map[string]string{"reset_url": m.consoleBase + "/reset-password?token=" + string(token)}
	return u.Email, "", data, nil
}

// PendingAccountEmails feeds the dispatcher's account-level outbox: reset tokens
// still valid with no terminal delivery. Account auth is not an org-spine fact,
// so these triggers come from their own table (mailer.AccountSource).
func (m *MailDirectory) PendingAccountEmails(ctx context.Context) ([]mailer.AccountTrigger, error) {
	ids, err := m.q.ListPendingResetEmails(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]mailer.AccountTrigger, 0, len(ids))
	for _, id := range ids {
		out = append(out, mailer.AccountTrigger{ID: id, Action: "password.reset_requested", Subject: id})
	}
	return out, nil
}
