package identity

// T13.3 — the AI assistant conversation store. Threads + messages persist
// independently of the ai-assistant policy: when that policy is `disabled`
// the surface is HIDDEN (the handlers return 404 empty-equivalent, AI Law 4),
// but nothing is deleted — re-enabling restores every thread verbatim.
//
// This file also carries the ai-assistant ENABLEMENT check (the reusable
// enforcement primitive the whole /assistant/* surface gates on — the T12.3
// contract): a single source of truth for "is the AI layer on for this org".

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
)

// AIAssistantEnabled reports whether the AI layer is on for an org. Disabled
// is the ONLY off state (AI Law 4); `enabled`/`opt_in`/absent all permit
// (opt-in is a presentation concern, not a denial). Closest-wins is not needed
// here — the ai-assistant policy is org-wide by contract (no project scope).
func (s *Service) AIAssistantEnabled(ctx context.Context, orgID string) (bool, error) {
	rows, err := s.q.ListPoliciesForOrg(ctx, orgID)
	if err != nil {
		return false, err
	}
	for _, p := range rows {
		if p.Key == "ai-assistant" && p.Enforcement == "disabled" {
			return false, nil
		}
	}
	return true, nil
}

// Thread is the storage-neutral thread with its ordered messages (messages nil
// unless the caller asked to hydrate them).
type Thread struct {
	Row      store.AssistantThread
	Messages []store.AssistantMessage
}

// CreateThread opens a conversation scoped to an org the caller belongs to.
// Callers MUST have passed the AIAssistantEnabled gate first (the handler
// does) — this is the store write, not the policy decision.
func (s *Service) CreateThread(ctx context.Context, orgID, userID, title string, context json.RawMessage, attachedInsight string) (store.AssistantThread, error) {
	if len(context) == 0 {
		context = json.RawMessage("{}")
	}
	attached := pgtype.Text{}
	if attachedInsight != "" {
		attached = pgtype.Text{String: attachedInsight, Valid: true}
	}
	return s.q.CreateThread(ctx, store.CreateThreadParams{
		ID: ids.New("thr"), OrgID: orgID, UserID: userID,
		Title: title, Context: context, AttachedInsight: attached,
	})
}

// ListThreads returns the caller's threads in one org, newest first. The
// disable gate is the handler's job — a disabled org is never queried, so its
// threads are hidden-not-deleted by construction.
func (s *Service) ListThreads(ctx context.Context, userID, orgID string, limit int32) ([]store.AssistantThread, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.q.ListThreadsForUserInOrg(ctx, store.ListThreadsForUserInOrgParams{
		UserID: userID, OrgID: orgID, Limit: limit,
	})
}
