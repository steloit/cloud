package identity

// T10.3 the in-app bell (U8) + per-user routing prefs (P6). All /me — the
// signed-in caller's own notifications; ids another user owns are never
// touched or probed (WHERE user_id).

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/httpapi/gen"
	"github.com/steloit/cloud/services/api/internal/identity/store"
)

// notifPageSize bounds one bell page (the contract has no client limit param).
const notifPageSize = 50

func (h *Handlers) ListNotifications(ctx context.Context, req gen.ListNotificationsRequestObject) (gen.ListNotificationsResponseObject, error) {
	p, err := requireUser(ctx, false)
	if err != nil {
		return nil, err
	}
	arg := store.ListNotificationsParams{UserID: p.UserID, Limit: notifPageSize + 1}
	if req.Params.Unread != nil {
		arg.Unread = pgtype.Bool{Bool: *req.Params.Unread, Valid: true}
	}
	if req.Params.Cursor != nil && *req.Params.Cursor != "" {
		arg.Cursor = pgtype.Text{String: *req.Params.Cursor, Valid: true}
	}
	rows, err := h.svc.q.ListNotifications(ctx, arg)
	if err != nil {
		return nil, err
	}
	// keyset: a full+1 page means there's a next; the extra row is the cursor.
	var next *string
	if len(rows) > notifPageSize {
		cur := rows[notifPageSize-1].ID
		next = &cur
		rows = rows[:notifPageSize]
	}
	type item = struct {
		Body      *string   `json:"body,omitempty"`
		CreatedAt time.Time `json:"created_at"`
		Id        string    `json:"id"`
		Kind      string    `json:"kind"`
		Link      *string   `json:"link,omitempty"`
		Read      bool      `json:"read"`
		Title     string    `json:"title"`
	}
	data := make([]item, 0, len(rows))
	for _, n := range rows {
		it := item{Id: n.ID, Kind: n.Kind, Title: n.Title, Read: n.Read, CreatedAt: n.CreatedAt.Time}
		if n.Body != "" {
			b := n.Body
			it.Body = &b
		}
		if n.Link != "" {
			l := n.Link
			it.Link = &l
		}
		data = append(data, it)
	}
	return gen.ListNotifications200JSONResponse(gen.NotificationList{Data: &data, NextCursor: next}), nil
}

func (h *Handlers) MarkNotificationsRead(ctx context.Context, req gen.MarkNotificationsReadRequestObject) (gen.MarkNotificationsReadResponseObject, error) {
	p, err := requireUser(ctx, true)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || len(req.Body.Ids) == 0 {
		return gen.MarkNotificationsRead204Response{}, nil // nothing to do
	}
	// scoped to the owner: ids the caller doesn't own are silently untouched.
	if err := h.svc.q.MarkNotificationsRead(ctx, store.MarkNotificationsReadParams{
		UserID: p.UserID, Ids: req.Body.Ids,
	}); err != nil {
		return nil, err
	}
	return gen.MarkNotificationsRead204Response{}, nil
}

func (h *Handlers) GetNotificationPrefs(ctx context.Context, _ gen.GetNotificationPrefsRequestObject) (gen.GetNotificationPrefsResponseObject, error) {
	p, err := requireUser(ctx, false)
	if err != nil {
		return nil, err
	}
	row, err := h.svc.q.GetNotificationPrefs(ctx, p.UserID)
	if err == pgx.ErrNoRows {
		return gen.GetNotificationPrefs200JSONResponse(defaultPrefs()), nil
	}
	if err != nil {
		return nil, err
	}
	return gen.GetNotificationPrefs200JSONResponse(prefsToAPI(row)), nil
}

func (h *Handlers) UpdateNotificationPrefs(ctx context.Context, req gen.UpdateNotificationPrefsRequestObject) (gen.UpdateNotificationPrefsResponseObject, error) {
	p, err := requireUser(ctx, true)
	if err != nil {
		return nil, err
	}
	// PATCH merge: load the STORED prefs once (zero row ⇒ defaults) and overlay
	// only the fields the caller actually sent — an unmentioned channel is never
	// silently flipped. A nil sub-field means "leave as-is", not "reset to on".
	cur, err := h.svc.q.GetNotificationPrefs(ctx, p.UserID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	api := prefsToAPI(cur) // defaults when the row is absent
	email, inapp := *api.Channels.Email, *api.Channels.Inapp
	quiet := cur.QuietHours // the stored raw jsonb (nil if none)
	if req.Body != nil {
		if req.Body.Channels != nil {
			if req.Body.Channels.Email != nil {
				email = *req.Body.Channels.Email
			}
			if req.Body.Channels.Inapp != nil {
				inapp = *req.Body.Channels.Inapp
			}
		}
		// a present quiet_hours object replaces; absent leaves the stored value
		// (an explicit null to CLEAR is indistinguishable from absent in the
		// generated pointer type — recorded as a follow-up).
		if req.Body.QuietHours != nil {
			quiet, _ = json.Marshal(req.Body.QuietHours)
		}
	}
	channels, _ := json.Marshal(map[string]bool{"email": email, "inapp": inapp})
	row, err := h.svc.q.UpsertNotificationPrefs(ctx, store.UpsertNotificationPrefsParams{
		UserID: p.UserID, Channels: channels, QuietHours: quiet,
	})
	if err != nil {
		return nil, err
	}
	return gen.UpdateNotificationPrefs200JSONResponse(prefsToAPI(row)), nil
}

func defaultPrefs() gen.NotificationPrefs {
	on := true
	return gen.NotificationPrefs{Channels: &struct {
		Email *bool `json:"email,omitempty"`
		Inapp *bool `json:"inapp,omitempty"`
	}{Email: &on, Inapp: &on}}
}

func prefsToAPI(row store.NotificationPref) gen.NotificationPrefs {
	out := defaultPrefs()
	if len(row.Channels) > 0 {
		var ch struct {
			Email *bool `json:"email"`
			Inapp *bool `json:"inapp"`
		}
		if json.Unmarshal(row.Channels, &ch) == nil {
			if ch.Email != nil {
				out.Channels.Email = ch.Email
			}
			if ch.Inapp != nil {
				out.Channels.Inapp = ch.Inapp
			}
		}
	}
	if len(row.QuietHours) > 0 && string(row.QuietHours) != "null" {
		var q struct {
			Start    *string `json:"start"`
			End      *string `json:"end"`
			Timezone *string `json:"timezone"`
		}
		if json.Unmarshal(row.QuietHours, &q) == nil && q.Start != nil {
			out.QuietHours = &struct {
				End      *string `json:"end,omitempty"`
				Start    *string `json:"start,omitempty"`
				Timezone *string `json:"timezone,omitempty"`
			}{End: q.End, Start: q.Start, Timezone: q.Timezone}
		}
	}
	return out
}
