package events

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/identity/store"
)

func TestCursorRoundTrip(t *testing.T) {
	at := time.Date(2026, 7, 18, 12, 30, 45, 123456000, time.UTC)
	enc := EncodeCursor(at, "evt_abc")
	cur, err := DecodeCursor(enc)
	if err != nil {
		t.Fatal(err)
	}
	if !cur.At.Equal(at) || cur.ID != "evt_abc" {
		t.Fatalf("round trip lost data: %+v", cur)
	}
	for _, bad := range []string{"", "!!!", "bm9wZQ"} {
		if _, err := DecodeCursor(bad); err == nil {
			t.Fatalf("malformed cursor %q accepted", bad)
		}
	}
}

func TestHubFanoutAndDrop(t *testing.T) {
	h := NewHub()
	a, cancelA := h.Subscribe("org_1")
	defer cancelA()
	b, cancelB := h.Subscribe("org_2")
	defer cancelB()

	h.Publish(store.Event{ID: "evt_1", OrgID: "org_1"})
	select {
	case evt := <-a:
		if evt.ID != "evt_1" {
			t.Fatalf("wrong event: %s", evt.ID)
		}
	default:
		t.Fatal("subscriber missed its org's event")
	}
	select {
	case evt := <-b:
		t.Fatalf("cross-org leak: org_2 subscriber got %s", evt.ID)
	default:
	}

	// A full subscriber is dropped, never blocks Publish.
	slow, cancelSlow := h.Subscribe("org_1")
	defer cancelSlow()
	done := make(chan struct{})
	go func() {
		for i := 0; i < subscriberBuffer+10; i++ {
			h.Publish(store.Event{ID: "evt_flood", OrgID: "org_1"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on a slow subscriber")
	}
	_ = slow

	// cancel is idempotent and closes the channel.
	cancelA()
	cancelA()
	if _, open := <-drain(a); open {
		t.Fatal("cancelled subscriber channel not closed")
	}
}

func drain(c <-chan store.Event) <-chan store.Event {
	for {
		select {
		case _, open := <-c:
			if !open {
				closed := make(chan store.Event)
				close(closed)
				return closed
			}
		default:
			return c
		}
	}
}

func TestEventsPathEnv(t *testing.T) {
	cases := map[string]struct {
		env string
		ok  bool
	}{
		"/v1/envs/env_1/events":   {"env_1", true},
		"/v1/envs/env_1/metrics":  {"", false},
		"/v1/envs//events":        {"", false},
		"/v1/envs/a/b/events":     {"", false},
		"/v1/orgs/org_1/audit":    {"", false},
		"/v1/envs/env_1/events/x": {"", false},
	}
	for path, want := range cases {
		env, ok := eventsPathEnv(path)
		if ok != want.ok || env != want.env {
			t.Fatalf("%s → (%q,%v), want (%q,%v)", path, env, ok, want.env, want.ok)
		}
	}
}

func TestAfterOrdering(t *testing.T) {
	base := time.Now()
	cur := Cursor{At: base, ID: "evt_b"}
	mk := func(at time.Time, id string) store.Event {
		return store.Event{ID: id, At: pgtype.Timestamptz{Time: at, Valid: true}}
	}
	if after(mk(base.Add(-time.Second), "evt_z"), cur) {
		t.Fatal("older event counted as after")
	}
	if after(mk(base, "evt_a"), cur) {
		t.Fatal("same-instant lower id counted as after")
	}
	if !after(mk(base, "evt_c"), cur) {
		t.Fatal("same-instant higher id not after")
	}
	if !after(mk(base.Add(time.Second), "evt_a"), cur) {
		t.Fatal("newer event not after")
	}
	if !after(mk(base.Add(-time.Hour), "x"), Cursor{}) {
		t.Fatal("empty cursor must pass everything")
	}
}
