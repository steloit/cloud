package provisioning

// T3.6: bindings — wiring, $0 (F3/U2). Credentials are MINTED at bind into
// the vault (never at rest in plaintext, never in a binding row) and ROTATED
// at unbind (a fresh version invalidates the old the moment the reconciler
// applies it). Status is `pending` immediately — effective next deploy.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
	"github.com/steloit/cloud/services/api/internal/secrets"
)

// EnvVarName is the deterministic injected name: <TARGET>_URL (U2).
func EnvVarName(targetName string) string {
	return strings.ToUpper(strings.ReplaceAll(targetName, "-", "_")) + "_URL"
}

// credentialFor mints the connection credential for an internal target. The
// hostname is the service's stable in-env name (grammar-only, D8); the
// reconciler materializes the actual user/grant at apply time.
func credentialFor(target store.Service, scope string) (string, error) {
	pw := make([]byte, 24)
	if _, err := rand.Read(pw); err != nil {
		return "", fmt.Errorf("provisioning: credential entropy: %w", err)
	}
	role := "rw"
	if scope == "read_only" {
		role = "ro"
	}
	switch target.Product {
	case "postgres":
		return fmt.Sprintf("postgres://bnd_%s:%s@%s.internal:5432/app", role, hex.EncodeToString(pw), target.Name), nil
	case "valkey":
		return fmt.Sprintf("redis://:%s@%s.internal:6379", hex.EncodeToString(pw), target.Name), nil
	default:
		return fmt.Sprintf("http://%s.internal:8080", target.Name), nil
	}
}

// CreateBinding — same-environment only (cross-env is a policy capability
// that does not exist yet; refused loudly, recorded finding).
func (s *Service) CreateBinding(ctx context.Context, source store.Service, targetID, scope, orgID, envID, projectID, actorID string) (store.Binding, string, error) {
	if scope == "" {
		scope = "read_only"
	}
	if targetID == source.ID {
		return store.Binding{}, "", problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "target", Detail: "a service cannot bind to itself"}})}
	}
	target, err := s.q.GetService(ctx, targetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Binding{}, "", notFound("target service")
	}
	if err != nil {
		return store.Binding{}, "", err
	}
	if target.EnvID != source.EnvID {
		return store.Binding{}, "", problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "target", Detail: "target must be in the same environment; cross-environment bindings are policy-gated and not yet available"}})}
	}

	cred, err := credentialFor(target, scope)
	if err != nil {
		return store.Binding{}, "", err
	}
	bindingID := ids.New("bnd")
	secretName := "binding/" + bindingID
	if _, err := s.vault.Put(ctx, secrets.Scope{OrgID: orgID, ProjectID: projectID, EnvID: envID}, secretName, []byte(cred), actorID); err != nil {
		return store.Binding{}, "", err
	}
	var intent pgtype.Text
	if target.Intent.Valid {
		intent = target.Intent
	}
	row, err := s.q.InsertBinding(ctx, store.InsertBindingParams{
		ID: bindingID, SourceID: source.ID,
		TargetID: pgtype.Text{String: target.ID, Valid: true},
		Intent:   intent, Scope: scope,
		SecretRef: pgtype.Text{String: secretName, Valid: true},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return store.Binding{}, "", problemError{p: problem.Conflict(
				[]string{"this binding already exists"},
				"One binding per source→target pair; change its scope by unbinding and rebinding.")}
		}
		return store.Binding{}, "", err
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "lifecycle", Via: "user", Actor: actorID,
		Action: "binding.created", Subject: row.ID,
		Detail: []byte(`{"source":` + strconv.Quote(source.Name) + `,"target":` + strconv.Quote(target.Name) + `,"scope":` + strconv.Quote(scope) + `}`),
	})
	return row, EnvVarName(target.Name), nil
}

func (s *Service) ListBindings(ctx context.Context, sourceID string) ([]store.Binding, error) {
	return s.q.ListBindingsForSource(ctx, sourceID)
}

// BindingOrg resolves binding → source service → org (404, no probing).
func (s *Service) BindingOrg(ctx context.Context, bindingID string) (store.Binding, store.Service, string, error) {
	b, err := s.q.GetBinding(ctx, bindingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Binding{}, store.Service{}, "", notFound("binding")
	}
	if err != nil {
		return store.Binding{}, store.Service{}, "", err
	}
	svc, orgID, err := s.ServiceOrg(ctx, b.SourceID)
	if err != nil {
		return store.Binding{}, store.Service{}, "", err
	}
	return b, svc, orgID, nil
}

// DeleteBinding — unbind ROTATES immediately: a fresh credential version is
// minted (the old one dies at the next reconcile) and then the vault entry
// is removed with the binding revoked. The row survives as `revoked` for the
// audit trail; the partial-unique index frees the pair for rebinding.
func (s *Service) DeleteBinding(ctx context.Context, b store.Binding, orgID, envID, projectID, actorID string) error {
	row, err := s.q.RevokeBinding(ctx, b.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notFound("binding")
		}
		return err
	}
	if row.SecretRef.Valid {
		// rotate-then-delete: even a racing reader now sees a dead credential
		if b.TargetID.Valid {
			if target, err := s.q.GetService(ctx, b.TargetID.String); err == nil {
				if fresh, err := credentialFor(target, row.Scope); err == nil {
					_, _ = s.vault.Put(ctx, secrets.Scope{OrgID: orgID, ProjectID: projectID, EnvID: envID}, row.SecretRef.String, []byte(fresh), actorID)
				}
			}
		}
		if err := s.vault.Delete(ctx, secrets.Scope{OrgID: orgID, ProjectID: projectID, EnvID: envID}, row.SecretRef.String); err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return err
		}
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "lifecycle", Via: "user", Actor: actorID,
		Action: "binding.revoked", Subject: b.ID,
		Detail: []byte(`{"rotated":true}`),
	})
	return nil
}
