package identity_test

// US-10.3: the bell projection against a real DB.
//
// The load-bearing property is that the projection cannot starve and cannot
// silently drop a member. Both failure modes were reached by earlier designs
// during review, so each has its own test here rather than riding on another's
// assertions.

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/notify"
)

// bellWorld is an org with a deployable fixture: the render context for
// deploy.created needs a real deployment → environment, and a named actor.
type bellWorld struct {
	*world
	q       *store.Queries
	router  *notify.Router
	orgID   string
	envName string
	depID   string
	ownerID string
}

func newBellWorld(t *testing.T, ownerEmail, orgSlug string) (*bellWorld, string) {
	t.Helper()
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	q := store.New(w.pool)

	ownerCk, ownerID := w.signupUser(t, ownerEmail)
	if r, b := w.post(t, "/v1/orgs", fmt.Sprintf(`{"name":%q}`, orgSlug), ownerCk); r.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", r.StatusCode, b)
	}
	var orgID string
	if err := w.pool.QueryRow(ctx, "select id from orgs where slug=$1", orgSlug).Scan(&orgID); err != nil {
		t.Fatal(err)
	}

	// The renderer reads users.name; the column defaults to '' and an empty name
	// makes the frame unrenderable by design, so give the actor one.
	if _, err := w.pool.Exec(ctx, "update users set name='priya' where id=$1", ownerID); err != nil {
		t.Fatal(err)
	}

	// Minimal project → env → service → deployment for the render context.
	prjID, envID, svcID, depID := "prj_bell", "env_bell", "svc_bell", "dep_bell"
	mustExec(t, w, `insert into projects (id, org_id, name) values ($1,$2,'shop')`, prjID, orgID)
	mustExec(t, w, `insert into environments (id, project_id, name) values ($1,$2,'production')`, envID, prjID)
	mustExec(t, w, `insert into services (id, env_id, name, product) values ($1,$2,'api','web')`, svcID, envID)
	mustExec(t, w, `insert into deployments (id, number, env_id, service_id, actor) values ($1,142,$2,$3,$4)`,
		depID, envID, svcID, ownerID)

	router := notify.NewRouter(q, w.kek).WithTxer(notify.NewPoolTxer(w.pool))
	return &bellWorld{world: w, q: q, router: router, orgID: orgID, envName: "production", depID: depID, ownerID: ownerID}, ownerCk
}

// addMember signs up a user and joins them to the org directly. The invite flow
// is exercised elsewhere; here the membership is a fixture, not the subject.
func (b *bellWorld) addMember(t *testing.T, email, orgID, _ string) (cookie, userID string) {
	t.Helper()
	ck, uid := b.signupUser(t, email)
	mustExec(t, b.world,
		`insert into members (id, org_id, user_id, role) values ($1,$2,$3,'developer')`,
		"mbr_"+uid[4:], orgID, uid)
	return ck, uid
}

func mustExec(t *testing.T, w *world, sql string, args ...any) {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %s: %v", sql, err)
	}
}

// appendEvent writes straight to the spine — the projection's only input.
func (b *bellWorld) appendEvent(t *testing.T, id, action, subject string) {
	t.Helper()
	mustExec(t, b.world,
		`insert into events (id, org_id, kind, via, actor, action, subject, detail)
		 values ($1,$2,'deploy','user',$3,$4,$5,'{}')`,
		id, b.orgID, b.ownerID, action, subject)
}

func (b *bellWorld) bellCount(t *testing.T, userID string) int {
	t.Helper()
	var n int
	if err := b.pool.QueryRow(context.Background(),
		`select count(*) from notifications where user_id=$1`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// scannedHas asks about ONE event. Counting rows would be wrong: creating the
// org and its members already appends spine events, so the ledger is never
// empty and a count conflates setup with the event under test.
func (b *bellWorld) scannedHas(t *testing.T, eventID string) bool {
	t.Helper()
	var n int
	if err := b.pool.QueryRow(context.Background(),
		`select count(*) from bell_scanned where event_id=$1`, eventID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

// TestUnclassifiedEventProducesNoBellRow — silence is the default.
func TestUnclassifiedEventProducesNoBellRow(t *testing.T) {
	b, _ := newBellWorld(t, "bell-unclass@example.com", "bellunclass")
	ctx := context.Background()

	b.appendEvent(t, "evt_unclass1", "org.updated", "org_x")

	if _, err := b.router.ProcessBell(ctx); err != nil {
		t.Fatalf("ProcessBell: %v", err)
	}
	if got := b.bellCount(t, b.ownerID); got != 0 {
		t.Fatalf("unclassified event rang the bell: %d rows, want 0", got)
	}
}

// TestUnclassifiedEventIsStillMarkedScanned — the half that prevents starvation.
// Without it a silent event stays in the window forever.
func TestUnclassifiedEventIsStillMarkedScanned(t *testing.T) {
	b, _ := newBellWorld(t, "bell-scanned@example.com", "bellscanned")
	ctx := context.Background()

	b.appendEvent(t, "evt_scan1", "org.updated", "org_x")

	if _, err := b.router.ProcessBell(ctx); err != nil {
		t.Fatalf("ProcessBell: %v", err)
	}
	if !b.scannedHas(t, "evt_scan1") {
		t.Fatal("an unworthy event was not marked scanned — the scan window will fill with it forever")
	}
	// And it must not come back on the next scan.
	pending, err := b.q.ListPendingBellEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range pending {
		if e.ID == "evt_scan1" {
			t.Fatal("an already-scanned event is still pending — the scan will starve")
		}
	}
}

// TestBellScanDoesNotHeadOfLineBlock — more than one full batch (LIMIT 100) of
// silent events must not bury a worthy one. This is the exact failure the first
// two designs had.
func TestBellScanDoesNotHeadOfLineBlock(t *testing.T) {
	b, _ := newBellWorld(t, "bell-hol@example.com", "bellhol")
	ctx := context.Background()

	for i := 0; i < 150; i++ {
		b.appendEvent(t, fmt.Sprintf("evt_noise%03d", i), "org.updated", "org_x")
	}
	b.appendEvent(t, "evt_worthy", "deploy.created", b.depID)

	// Drain: 150 silent + 1 worthy needs more than one batch.
	for i := 0; i < 5; i++ {
		if _, err := b.router.ProcessBell(ctx); err != nil {
			t.Fatalf("ProcessBell: %v", err)
		}
	}
	if got := b.bellCount(t, b.ownerID); got != 1 {
		t.Fatalf("worthy event behind 150 silent ones produced %d bell rows, want 1 "+
			"— the scan window is being poisoned", got)
	}
}

// TestWorthyEventFansToEveryOptedInMember — every opted-in member, exactly once;
// opted-out members get nothing. Also covers the rendered title being the frame's.
func TestWorthyEventFansToEveryOptedInMember(t *testing.T) {
	b, ownerCk := newBellWorld(t, "bell-fan@example.com", "bellfan")
	ctx := context.Background()

	// A second member with in-app OFF, and a third inside an active quiet
	// window — quiet hours gate email only and must never suppress a bell row.
	offCk, offID := b.addMember(t, "bell-fan-off@example.com", b.orgID, ownerCk)
	if r, _ := b.patch(t, "/v1/me/notification-prefs", `{"channels":{"inapp":false}}`, offCk); r.StatusCode != 200 {
		t.Fatal("set inapp off")
	}
	quietCk, quietID := b.addMember(t, "bell-fan-quiet@example.com", b.orgID, ownerCk)
	if r, _ := b.patch(t, "/v1/me/notification-prefs",
		`{"quiet_hours":{"start":"00:00","end":"23:59","timezone":"UTC"}}`, quietCk); r.StatusCode != 200 {
		t.Fatal("set quiet hours")
	}

	b.appendEvent(t, "evt_fan1", "deploy.created", b.depID)
	if n, err := b.router.ProcessBell(ctx); err != nil || n != 1 {
		t.Fatalf("ProcessBell = %d, %v; want 1, nil", n, err)
	}

	if got := b.bellCount(t, b.ownerID); got != 1 {
		t.Fatalf("owner bell rows = %d, want 1", got)
	}
	if got := b.bellCount(t, quietID); got != 1 {
		t.Fatalf("quiet-hours member bell rows = %d, want 1 — quiet hours route email, never the bell", got)
	}
	if got := b.bellCount(t, offID); got != 0 {
		t.Fatalf("in-app-off member bell rows = %d, want 0", got)
	}

	// The persisted title is the frame's, rendered once (Option A).
	var title, kind string
	if err := b.pool.QueryRow(ctx,
		`select title, kind from notifications where user_id=$1`, b.ownerID).Scan(&title, &kind); err != nil {
		t.Fatal(err)
	}
	if want := "Deploy #142 to production by priya"; title != want {
		t.Fatalf("title = %q, want %q (frame N1 verbatim)", title, want)
	}
	if kind != notify.KindDeploy {
		t.Fatalf("kind = %q, want %q", kind, notify.KindDeploy)
	}
}

// TestBellProjectionIsIdempotent — a second run adds nothing.
func TestBellProjectionIsIdempotent(t *testing.T) {
	b, _ := newBellWorld(t, "bell-idem@example.com", "bellidem")
	ctx := context.Background()

	b.appendEvent(t, "evt_idem1", "deploy.created", b.depID)

	first, err := b.router.ProcessBell(ctx)
	if err != nil || first != 1 {
		t.Fatalf("first ProcessBell = %d, %v; want 1, nil", first, err)
	}
	after := b.bellCount(t, b.ownerID)

	second, err := b.router.ProcessBell(ctx)
	if err != nil {
		t.Fatalf("second ProcessBell: %v", err)
	}
	if second != 0 {
		t.Fatalf("second ProcessBell projected %d events, want 0", second)
	}
	if got := b.bellCount(t, b.ownerID); got != after {
		t.Fatalf("bell rows changed on re-run: %d → %d", after, got)
	}
}

// TestConcurrentProcessBellFansExactlyOnce — the claim makes two concurrent
// drains produce one fan-out, not two.
func TestConcurrentProcessBellFansExactlyOnce(t *testing.T) {
	b, _ := newBellWorld(t, "bell-conc@example.com", "bellconc")
	ctx := context.Background()

	b.appendEvent(t, "evt_conc1", "deploy.created", b.depID)

	var wg sync.WaitGroup
	total := make([]int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			n, err := b.router.ProcessBell(ctx)
			if err != nil {
				t.Errorf("concurrent ProcessBell: %v", err)
			}
			total[i] = n
		}(i)
	}
	wg.Wait()

	if sum := total[0] + total[1]; sum != 1 {
		t.Fatalf("concurrent drains projected %d events in total, want exactly 1", sum)
	}
	if got := b.bellCount(t, b.ownerID); got != 1 {
		t.Fatalf("bell rows = %d after concurrent drains, want 1", got)
	}
}

// failingNthStore fails the Nth InsertNotification, so the partial-fan-out
// rollback is observable. This is why Txer yields a Store, not *store.Queries.
type failingNthStore struct {
	notify.Store
	mu     sync.Mutex
	n      int
	failOn int
}

func (f *failingNthStore) InsertNotification(ctx context.Context, arg store.InsertNotificationParams) (store.Notification, error) {
	f.mu.Lock()
	f.n++
	hit := f.n == f.failOn
	f.mu.Unlock()
	if hit {
		return store.Notification{}, fmt.Errorf("injected insert failure")
	}
	return f.Store.InsertNotification(ctx, arg)
}

// failingTxer wraps a real Txer, substituting the failing store.
type failingTxer struct {
	inner  notify.Txer
	failOn int
	armed  bool
}

func (f *failingTxer) WithTx(ctx context.Context, fn func(notify.Store) error) error {
	return f.inner.WithTx(ctx, func(s notify.Store) error {
		if !f.armed {
			return fn(s)
		}
		return fn(&failingNthStore{Store: s, failOn: f.failOn})
	})
}

// TestPartialFanOutIsRetriedNotSwallowed — a mid-fan-out failure must roll the
// claim back so a later run notifies EVERY member. The design that committed
// the claim separately dropped members 4..10 of a 10-member org, silently and
// permanently.
func TestPartialFanOutIsRetriedNotSwallowed(t *testing.T) {
	b, ownerCk := newBellWorld(t, "bell-partial@example.com", "bellpartial")
	ctx := context.Background()

	_, m2 := b.addMember(t, "bell-partial-2@example.com", b.orgID, ownerCk)
	_, m3 := b.addMember(t, "bell-partial-3@example.com", b.orgID, ownerCk)

	b.appendEvent(t, "evt_partial1", "deploy.created", b.depID)

	// Fail the 2nd insert of 3 → the whole event's tx must unwind.
	ft := &failingTxer{inner: notify.NewPoolTxer(b.pool), failOn: 2, armed: true}
	broken := notify.NewRouter(b.q, b.kek).WithTxer(ft)
	if n, _ := broken.ProcessBell(ctx); n != 0 {
		t.Fatalf("failed fan-out reported %d projected, want 0", n)
	}
	for _, id := range []string{b.ownerID, m2, m3} {
		if got := b.bellCount(t, id); got != 0 {
			t.Fatalf("partial fan-out COMMITTED for %s (%d rows) — the claim did not roll back", id, got)
		}
	}
	if b.scannedHas(t, "evt_partial1") {
		t.Fatal("claim survived a rolled-back fan-out — the event will never be retried")
	}

	// Unbroken run must reach all three.
	if n, err := b.router.ProcessBell(ctx); err != nil || n != 1 {
		t.Fatalf("retry ProcessBell = %d, %v; want 1, nil", n, err)
	}
	for _, id := range []string{b.ownerID, m2, m3} {
		if got := b.bellCount(t, id); got != 1 {
			t.Fatalf("member %s has %d bell rows after retry, want 1 — a member was silently skipped", id, got)
		}
	}
}

// TestLateJoinerGetsNoBackfill — both halves: the new member gets nothing, and
// existing members are not re-notified.
func TestLateJoinerGetsNoBackfill(t *testing.T) {
	b, ownerCk := newBellWorld(t, "bell-late@example.com", "belllate")
	ctx := context.Background()

	b.appendEvent(t, "evt_late1", "deploy.created", b.depID)
	if _, err := b.router.ProcessBell(ctx); err != nil {
		t.Fatalf("ProcessBell: %v", err)
	}
	ownerBefore := b.bellCount(t, b.ownerID)

	_, lateID := b.addMember(t, "bell-late-2@example.com", b.orgID, ownerCk)
	if _, err := b.router.ProcessBell(ctx); err != nil {
		t.Fatalf("ProcessBell after join: %v", err)
	}

	if got := b.bellCount(t, lateID); got != 0 {
		t.Fatalf("late joiner received %d backfilled rows, want 0", got)
	}
	if got := b.bellCount(t, b.ownerID); got != ownerBefore {
		t.Fatalf("existing member re-notified: %d → %d", ownerBefore, got)
	}
}

// TestPreExistingSpineIsNotBackfilled — the migration seed, exercised against a
// NON-empty spine. CI migrates an empty DB, so a wrong seed predicate would
// otherwise ship silently, and its effect is irreversible in production.
func TestPreExistingSpineIsNotBackfilled(t *testing.T) {
	b, _ := newBellWorld(t, "bell-seed@example.com", "bellseed")
	ctx := context.Background()

	// Simulate history that predates the ledger: append events, then re-run the
	// seed exactly as the migration does.
	for i := 0; i < 3; i++ {
		b.appendEvent(t, fmt.Sprintf("evt_hist%d", i), "deploy.created", b.depID)
	}
	mustExec(t, b.world, `delete from bell_scanned`)
	mustExec(t, b.world, `insert into bell_scanned (event_id) select id from events on conflict do nothing`)

	n, err := b.router.ProcessBell(ctx)
	if err != nil {
		t.Fatalf("ProcessBell: %v", err)
	}
	if n != 0 {
		t.Fatalf("projected %d pre-existing events, want 0 — the seed must prevent mass backfill", n)
	}
	if got := b.bellCount(t, b.ownerID); got != 0 {
		t.Fatalf("history was fanned onto the bell: %d rows, want 0", got)
	}
}

func (b *bellWorld) unrenderableFlag(t *testing.T, eventID string) bool {
	t.Helper()
	var f bool
	if err := b.pool.QueryRow(context.Background(),
		`select unrenderable from bell_scanned where event_id=$1`, eventID).Scan(&f); err != nil {
		t.Fatalf("no ledger row for %s: %v", eventID, err)
	}
	return f
}

// TestUnrenderableWorthyEventIsSilentAndFlagged pins the "never invent copy"
// rule and — just as important — that the decision stays applicable later.
//
// The org-key case is not hypothetical: ADR-0007 org keys carry user_id NULL, so
// an automation deploy writes an empty actor to the spine and N1 supplies no
// frame for a non-human actor. Without the flag those events are
// indistinguishable from uninteresting ones and could never be re-projected once
// the copy is decided.
func TestUnrenderableWorthyEventIsSilentAndFlagged(t *testing.T) {
	cases := []struct {
		name    string
		eventID string
		slug    string
		arrange func(t *testing.T, b *bellWorld)
		actor   string
	}{
		{
			name: "actor has no name (org key / automation deploy)", eventID: "evt_unrend_actor", slug: "urnoactor",
			arrange: func(t *testing.T, b *bellWorld) {
				mustExec(t, b.world, `update users set name='' where id=$1`, b.ownerID)
			},
		},
		{
			name: "environment has no name", eventID: "evt_unrend_env", slug: "urnoenv",
			arrange: func(t *testing.T, b *bellWorld) {
				mustExec(t, b.world, `update environments set name='' where id='env_bell'`)
			},
		},
		{
			name: "deployment no longer exists", eventID: "evt_unrend_gone", slug: "urnodep",
			arrange: func(t *testing.T, b *bellWorld) {},
			actor:   "dep_does_not_exist",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := newBellWorld(t, "unrend-"+tc.eventID+"@example.com", tc.slug)
			ctx := context.Background()
			tc.arrange(t, b)

			subject := b.depID
			if tc.actor != "" {
				subject = tc.actor
			}
			b.appendEvent(t, tc.eventID, "deploy.created", subject)

			n, err := b.router.ProcessBell(ctx)
			if err != nil {
				t.Fatalf("ProcessBell: %v", err)
			}
			if n != 0 {
				t.Fatalf("projected %d, want 0 — an unrenderable frame must not ring", n)
			}
			if got := b.bellCount(t, b.ownerID); got != 0 {
				t.Fatalf("bell rows = %d, want 0 — no copy may be invented", got)
			}
			if !b.scannedHas(t, tc.eventID) {
				t.Fatal("unrenderable event was not marked scanned — the scan would refill with it")
			}
			if !b.unrenderableFlag(t, tc.eventID) {
				t.Fatal("event marked scanned but NOT flagged unrenderable — it is now " +
					"indistinguishable from an uninteresting event and can never be re-projected")
			}
		})
	}
}

// TestPreExistingBellRowIsBenignDedupe — pgx.ErrNoRows from InsertNotification is
// the partial unique index deduping, not a failure. Inverting that check leaves
// every other test green, so this is the only guard on it.
func TestPreExistingBellRowIsBenignDedupe(t *testing.T) {
	b, ownerCk := newBellWorld(t, "bell-dedupe@example.com", "belldedupe")
	ctx := context.Background()
	_, m2 := b.addMember(t, "bell-dedupe-2@example.com", b.orgID, ownerCk)

	b.appendEvent(t, "evt_dedupe", "deploy.created", b.depID)
	// Pre-existing row for ONE member: their insert will conflict.
	mustExec(t, b.world,
		`insert into notifications (id, user_id, event_id, kind, title) values ('ntf_pre',$1,'evt_dedupe','deploy','pre-existing')`,
		b.ownerID)

	n, err := b.router.ProcessBell(ctx)
	if err != nil {
		t.Fatalf("ProcessBell: %v", err)
	}
	if n != 1 {
		t.Fatalf("projected %d, want 1 — a benign dedupe must not abort the fan-out", n)
	}
	if got := b.bellCount(t, b.ownerID); got != 1 {
		t.Fatalf("owner rows = %d, want 1 (the pre-existing one, not a duplicate)", got)
	}
	if got := b.bellCount(t, m2); got != 1 {
		t.Fatalf("second member rows = %d, want 1 — one member's dedupe aborted the whole fan-out", got)
	}
}

// TestBellDoesNotCrossOrgs — the render context is org-fenced, so an event can
// never render another tenant's environment name into a title.
func TestBellDoesNotCrossOrgs(t *testing.T) {
	b, _ := newBellWorld(t, "bell-orgA@example.com", "bellorga")
	ctx := context.Background()

	// A second org with its own project/env/deployment.
	otherCk, otherID := b.signupUser(t, "bell-orgB@example.com")
	if r, body := b.post(t, "/v1/orgs", `{"name":"bellorgb"}`, otherCk); r.StatusCode != 201 {
		t.Fatalf("createOrg B: %d %s", r.StatusCode, body)
	}
	var orgB string
	if err := b.pool.QueryRow(ctx, "select id from orgs where slug='bellorgb'").Scan(&orgB); err != nil {
		t.Fatal(err)
	}
	mustExec(t, b.world, `insert into projects (id, org_id, name) values ('prj_b',$1,'shopb')`, orgB)
	mustExec(t, b.world, `insert into environments (id, project_id, name) values ('env_b','prj_b','staging')`)
	mustExec(t, b.world, `insert into services (id, env_id, name, product) values ('svc_b','env_b','api','web')`)
	mustExec(t, b.world, `insert into deployments (id, number, env_id, service_id, actor) values ('dep_b',7,'env_b','svc_b',$1)`, otherID)

	// Org A's event pointing at org B's deployment must NOT render.
	b.appendEvent(t, "evt_cross", "deploy.created", "dep_b")
	if _, err := b.router.ProcessBell(ctx); err != nil {
		t.Fatalf("ProcessBell: %v", err)
	}
	if got := b.bellCount(t, b.ownerID); got != 0 {
		t.Fatalf("org A got %d bell rows from org B's deployment — the render context is not fenced", got)
	}
	if got := b.bellCount(t, otherID); got != 0 {
		t.Fatalf("org B member received %d rows from org A's event", got)
	}
	var leaked int
	if err := b.pool.QueryRow(ctx, `select count(*) from notifications where title like '%staging%'`).Scan(&leaked); err != nil {
		t.Fatal(err)
	}
	if leaked != 0 {
		t.Fatalf("org B's environment name leaked into %d notification titles", leaked)
	}
}
