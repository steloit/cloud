package identity

import (
	"context"

	"github.com/steloit/cloud/services/api/internal/identity/policy"
	"github.com/steloit/cloud/services/api/internal/identity/store"
)

// PolicySource adapts the sqlc store into policy.Source, keeping the policy
// engine storage-agnostic (it evaluates rows; it never queries).
type PolicySource struct{ q *store.Queries }

func NewPolicySource(q *store.Queries) *PolicySource { return &PolicySource{q: q} }

func (s *PolicySource) PoliciesForOrg(ctx context.Context, orgID string) ([]policy.Row, error) {
	stored, err := s.q.ListPoliciesForOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	rows := make([]policy.Row, 0, len(stored))
	for _, p := range stored {
		rows = append(rows, policy.Row{
			ID:          p.ID,
			OrgID:       p.OrgID,
			ProjectID:   p.ProjectID.String, // pgtype: invalid (NULL) yields "" = org-wide
			Key:         p.Key,
			Enforcement: p.Enforcement,
			Config:      p.Config,
		})
	}
	return rows, nil
}
