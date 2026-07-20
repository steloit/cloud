package identity_test

// US-10.2 — quiet hours affect ROUTING ONLY, never recording or firing. During
// a member's quiet window: the bell is STILL recorded (the durable record) and
// the underlying event STILL fires; only the best-effort EMAIL route is
// suppressed for NORMAL notifications. A security/paging-class (Urgent)
// notification is NEVER gated — its email routes even inside quiet hours.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/notify"
)

func TestQuietHoursRouteOnlyNeverRecordOrFire(t *testing.T) {
	w := newWorld(t, time.Hour)
	ctx := context.Background()
	ownerCk, _ := w.signupUser(t, "qh-owner@example.com")
	resp, body := w.post(t, "/v1/orgs", `{"name":"qhco"}`, ownerCk)
	if resp.StatusCode != 201 {
		t.Fatalf("createOrg: %d %s", resp.StatusCode, body)
	}
	var org struct{ Id string }
	_ = json.Unmarshal([]byte(body), &org)

	// email ON, quiet hours 22:00–07:00 UTC.
	if r, b := w.patch(t, "/v1/me/notification-prefs", `{"channels":{"email":true,"inapp":true}}`, ownerCk); r.StatusCode != 200 {
		t.Fatalf("prefs: %d %s", r.StatusCode, b)
	}
	if r, _ := w.patch(t, "/v1/me/notification-prefs", `{"quiet_hours":{"start":"22:00","end":"07:00","timezone":"UTC"}}`, ownerCk); r.StatusCode != 200 {
		t.Fatal("quiet hours")
	}

	// pin the clock INSIDE the quiet window (23:00 UTC) — deterministic.
	insideQuiet := func() time.Time { return time.Date(2026, 7, 15, 23, 0, 0, 0, time.UTC) }
	fake := &recordingSender{}
	router := notify.NewRouter(store.New(w.pool), w.kek).WithEmail(fake).WithClock(insideQuiet)

	// --- NORMAL notification inside quiet hours ------------------------------
	if err := router.Notify(ctx, notify.NotifyInput{
		OrgID: org.Id, Kind: "lifecycle", Title: "Deploy finished", Body: "svc ready",
	}); err != nil {
		t.Fatalf("notify normal: %v", err)
	}
	// RECORDING happened: the bell carries it.
	_, lb := w.get(t, "/v1/me/notifications", ownerCk)
	if !strings.Contains(lb, "Deploy finished") {
		t.Fatalf("quiet hours suppressed RECORDING (bell missing): %s", lb)
	}
	// ROUTING suppressed: no email during quiet hours for a normal notification.
	if fake.count() != 0 {
		t.Fatalf("normal email routed inside quiet hours (%d) — quiet hours must suppress the route", fake.count())
	}

	// --- SECURITY/PAGING (Urgent) notification inside quiet hours ------------
	if err := router.Notify(ctx, notify.NotifyInput{
		OrgID: org.Id, Kind: "security", Title: "Suspicious sign-in", Body: "new device", Urgent: true,
	}); err != nil {
		t.Fatalf("notify urgent: %v", err)
	}
	// recorded on the bell…
	_, lb = w.get(t, "/v1/me/notifications", ownerCk)
	if !strings.Contains(lb, "Suspicious sign-in") {
		t.Fatalf("urgent notification not recorded: %s", lb)
	}
	// …AND its email routed — quiet hours NEVER gate security/paging.
	if fake.count() != 1 {
		t.Fatalf("urgent (security/paging) email must route inside quiet hours (never gated), got %d", fake.count())
	}

	// --- OUTSIDE quiet hours, a normal notification DOES route --------------
	router = notify.NewRouter(store.New(w.pool), w.kek).WithEmail(fake).
		WithClock(func() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) })
	if err := router.Notify(ctx, notify.NotifyInput{
		OrgID: org.Id, Kind: "lifecycle", Title: "Daytime deploy", Body: "ok",
	}); err != nil {
		t.Fatal(err)
	}
	if fake.count() != 2 { // 1 urgent + this daytime normal
		t.Fatalf("a normal notification outside quiet hours must route: total emails %d (want 2)", fake.count())
	}
}
