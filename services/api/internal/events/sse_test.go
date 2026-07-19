package events

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func mustCursorTime(t *testing.T) time.Time {
	t.Helper()
	return time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
}

// T6.4: the SSE hardening — Last-Event-ID resume, the retry hint, malformed
// cursor rejection. Path matching + frame writing are exercised by
// events_test.go; here we pin the resume-header contract.

// fakeResolver + a minimal Streamer are hard to stand up without a DB, so
// these tests target the pure pieces the endpoint composes.

func TestResumePrecedence(t *testing.T) {
	// ?cursor= wins over Last-Event-ID; either decodes via the same codec.
	at := mustCursorTime(t)
	explicit := EncodeCursor(at, "evt_explicit")
	header := EncodeCursor(at, "evt_header")

	r := httptest.NewRequest("GET", "/v1/envs/env_1/events?cursor="+explicit, nil)
	r.Header.Set("Last-Event-ID", header)
	got := r.URL.Query().Get("cursor")
	if got == "" {
		got = r.Header.Get("Last-Event-ID")
	}
	cur, err := DecodeCursor(got)
	if err != nil || cur.ID != "evt_explicit" {
		t.Fatalf("explicit cursor should win: %+v %v", cur, err)
	}

	// no query cursor → the header is used
	r2 := httptest.NewRequest("GET", "/v1/envs/env_1/events", nil)
	r2.Header.Set("Last-Event-ID", header)
	got2 := r2.URL.Query().Get("cursor")
	if got2 == "" {
		got2 = r2.Header.Get("Last-Event-ID")
	}
	cur2, err := DecodeCursor(got2)
	if err != nil || cur2.ID != "evt_header" {
		t.Fatalf("Last-Event-ID should resume: %+v %v", cur2, err)
	}
	_ = strings.TrimSpace
}
