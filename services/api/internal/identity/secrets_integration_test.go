package identity_test

// T3.5: the vault against real Postgres — round-trip, versioning, scope
// isolation, and the load-bearing property: PLAINTEXT NEVER AT REST.

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/secrets"
)

func TestSecretsVault(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()

	_, ownerID := w.signupUser(t, "sec-owner@example.com")
	org, err := w.svc.CreateOrgWithOwner(ctx, "secco", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	orgScope := secrets.Scope{OrgID: org.ID}
	plaintext := []byte("postgres://user:hunter2-credential@host/db")

	// --- round-trip + versioning --------------------------------------------
	v1, err := w.vault.Put(ctx, orgScope, "db-cred", plaintext, ownerID)
	if err != nil || v1 != 1 {
		t.Fatalf("put v1: %d %v", v1, err)
	}
	got, ver, err := w.vault.Get(ctx, orgScope, "db-cred")
	if err != nil || !bytes.Equal(got, plaintext) || ver != 1 {
		t.Fatalf("get: %q v%d %v", got, ver, err)
	}
	rotated := []byte("postgres://user:rotated@host/db")
	v2, err := w.vault.Put(ctx, orgScope, "db-cred", rotated, ownerID)
	if err != nil || v2 != 2 {
		t.Fatalf("put v2: %d %v", v2, err)
	}
	got, ver, _ = w.vault.Get(ctx, orgScope, "db-cred")
	if !bytes.Equal(got, rotated) || ver != 2 {
		t.Fatalf("latest after rotation: %q v%d", got, ver)
	}

	// --- plaintext never at rest -------------------------------------------
	rows, err := w.pool.Query(ctx, "select ciphertext, wrapped_dek from secrets where org_id=$1", org.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var ct, wd []byte
		if err := rows.Scan(&ct, &wd); err != nil {
			t.Fatal(err)
		}
		for _, pt := range [][]byte{plaintext, rotated} {
			if bytes.Contains(ct, pt) || bytes.Contains(wd, pt) ||
				bytes.Contains(ct, []byte("hunter2")) {
				t.Fatal("plaintext found at rest")
			}
		}
		n++
	}
	if n != 2 {
		t.Fatalf("expected 2 versions at rest, got %d", n)
	}

	// --- scope isolation: org-scope ≠ env-scope for the same name ----------
	orgRow, err := w.svc.GetOrg(ctx, org.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := w.prov.CreateProject(ctx, orgRow, "vaulted", "", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	envScope := secrets.Scope{OrgID: org.ID, ProjectID: env.ProjectID, EnvID: env.ID}
	if _, _, err := w.vault.Get(ctx, envScope, "db-cred"); err != secrets.ErrNotFound {
		t.Fatalf("env-scope read of an org-scope secret: %v", err)
	}
	if _, err := w.vault.Put(ctx, envScope, "db-cred", []byte("env-local"), ownerID); err != nil {
		t.Fatal(err)
	}
	got, _, _ = w.vault.Get(ctx, envScope, "db-cred")
	if string(got) != "env-local" {
		t.Fatalf("env-scope secret: %q", got)
	}
	got, _, _ = w.vault.Get(ctx, orgScope, "db-cred")
	if !bytes.Equal(got, rotated) {
		t.Fatal("scopes bled into each other")
	}

	// --- a row copied across scopes fails authentication (AAD) -------------
	if _, err := w.pool.Exec(ctx,
		`update secrets set env_id = $1, project_id = $2 where org_id = $3 and env_id is null and name = 'db-cred' and version = 2`,
		env.ID, env.ProjectID, org.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := w.vault.Get(ctx, envScope, "db-cred"); err == nil {
		t.Fatal("scope-moved ciphertext decrypted — AAD not binding")
	}

	// --- delete every version ----------------------------------------------
	if err := w.vault.Delete(ctx, secrets.Scope{OrgID: org.ID}, "db-cred"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := w.vault.Get(ctx, secrets.Scope{OrgID: org.ID}, "db-cred"); err != secrets.ErrNotFound {
		t.Fatalf("deleted secret readable: %v", err)
	}
}
