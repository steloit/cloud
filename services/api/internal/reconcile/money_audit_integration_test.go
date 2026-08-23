package reconcile_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The O20 detection queries must actually FIND a poisoned row. A query that
// looks right and matches nothing reports the database clean, which is the
// failure this whole family of tasks is about.
//
// It also pins the queries to the doc: they are extracted from
// docs/dev/money-range-audit.md rather than retyped here, so the runnable
// version and the version support copy-pastes cannot drift apart.
func TestO20DetectionQueriesFindPoisonedRows(t *testing.T) {
	pool, _ := realDB(t)
	ctx := context.Background()

	// The bound is DERIVED in money.go; if this literal ever disagrees with the
	// doc, the doc is what support runs, so the doc is what is tested.
	const bound = "3443612618300"
	doc := readAuditDoc(t)
	if !strings.Contains(doc, bound) {
		t.Fatalf("docs/dev/money-range-audit.md no longer uses the bound %s — "+
			"money.MaxMonthly changed and the doc support runs is now wrong", bound)
	}

	seed := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	seed(`INSERT INTO orgs (id, name, slug, plan) VALUES ('org_o20','o20','o20','free')
	      ON CONFLICT (id) DO NOTHING`)

	// One poisoned row per storage location, both DIRECTIONS where the column
	// allows it — negative is what the overflow actually produced, positive
	// out-of-range is what a wrapped aggregate produces.
	seed(`INSERT INTO budgets (org_id, limit_cents) VALUES ('org_o20', -1)
	      ON CONFLICT (org_id) DO UPDATE SET limit_cents = -1`)
	seed(`INSERT INTO estimates (id, org_id, env_id, services, lines, total_cents, expires_at)
	      VALUES ('est_o20','org_o20',NULL,'[]'::jsonb,
	              '[{"name":"poisoned","monthly_cents":-5340232221128652948}]'::jsonb,
	              7766279631452245720, now() + interval '1 hour')
	      ON CONFLICT (id) DO NOTHING`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM estimates WHERE id='est_o20'`)
		_, _ = pool.Exec(context.Background(), `DELETE FROM budgets WHERE org_id='org_o20'`)
		_, _ = pool.Exec(context.Background(), `DELETE FROM orgs WHERE id='org_o20'`)
	})

	// --- the scalar sweep, extracted from the doc -------------------------
	scalar := extractSQL(t, doc, 0)
	rows, err := pool.Query(ctx, scalar)
	if err != nil {
		t.Fatalf("the documented scalar query does not run: %v\n%s", err, scalar)
	}
	found := map[string]bool{}
	for rows.Next() {
		var src, id, org string
		var cents int64
		if err := rows.Scan(&src, &id, &org, &cents); err != nil {
			t.Fatal(err)
		}
		found[src] = true
	}
	rows.Close()
	for _, want := range []string{"budgets", "estimates.total"} {
		if !found[want] {
			t.Errorf("the documented scalar query did not find the poisoned %s row — "+
				"a query that matches nothing reports the database CLEAN", want)
		}
	}

	// --- the jsonb sweep, which is NOT a duplicate of the scalar one -------
	// estimates.total_cents above is out of range too, but a line can be
	// poisoned while the total is fine; that is why the doc has two queries.
	jsonbQ := extractSQL(t, doc, 1)
	var lines int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM (`+strings.TrimSuffix(strings.TrimSpace(jsonbQ), ";")+`) x`).Scan(&lines); err != nil {
		t.Fatalf("the documented jsonb query does not run: %v\n%s", err, jsonbQ)
	}
	if lines == 0 {
		t.Error("the documented jsonb query did not find the poisoned estimates.lines entry")
	}

	// --- negative case: a clean row must NOT be reported -------------------
	seed(`INSERT INTO budgets (org_id, limit_cents) VALUES ('org_o20', 10000)
	      ON CONFLICT (org_id) DO UPDATE SET limit_cents = 10000`)
	var stillFlagged int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM budgets WHERE org_id='org_o20'
		   AND (limit_cents < 0 OR limit_cents > `+bound+`)`).Scan(&stillFlagged); err != nil {
		t.Fatal(err)
	}
	if stillFlagged != 0 {
		t.Error("an in-range budget was flagged — the query has a false positive, which would send support chasing healthy rows")
	}
}

func readAuditDoc(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "dev", "money-range-audit.md"))
	if err != nil {
		t.Fatalf("cannot read the audit doc: %v — this test must not pass by failing to look", err)
	}
	return string(b)
}

// extractSQL pulls the nth ```sql block out of the doc, so the runnable query
// and the one support copy-pastes are the same text.
func extractSQL(t *testing.T, doc string, n int) string {
	t.Helper()
	blocks := regexp.MustCompile("(?s)```sql\n(.*?)```").FindAllStringSubmatch(doc, -1)
	if len(blocks) <= n {
		t.Fatalf("docs/dev/money-range-audit.md has %d sql blocks, wanted at least %d", len(blocks), n+1)
	}
	return blocks[n][1]
}
