package notify

import (
	"testing"
	"time"
)

func TestSafeURL(t *testing.T) {
	bad := []string{
		"http://example.com/hook",      // not https
		"https://127.0.0.1/hook",       // loopback
		"https://localhost/hook",       // loopback by name
		"https://10.0.0.5/hook",        // private
		"https://169.254.169.254/meta", // link-local (cloud metadata)
		"https://[::1]/hook",           // loopback v6
		"ftp://example.com",            // wrong scheme
		"://nonsense",                  // unparseable host
	}
	for _, u := range bad {
		if err := safeURL(u, false); err == nil {
			t.Errorf("safeURL accepted an unsafe target: %s", u)
		}
	}
	// the insecure test knob relaxes scheme + address (loopback for httptest).
	if err := safeURL("http://127.0.0.1:8080/hook", true); err != nil {
		t.Errorf("insecure safeURL rejected a loopback target: %v", err)
	}
}

func TestQuietNow(t *testing.T) {
	at := func(hm string) time.Time {
		tt, _ := time.Parse("15:04", hm)
		return time.Date(2026, 1, 1, tt.Hour(), tt.Minute(), 0, 0, time.UTC)
	}
	cases := []struct {
		name      string
		q         *quietHours
		now       string
		wantQuiet bool
	}{
		{"no window", nil, "03:00", false},
		{"same-day inside", &quietHours{start: "09:00", end: "17:00"}, "12:00", true},
		{"same-day before", &quietHours{start: "09:00", end: "17:00"}, "08:59", false},
		{"same-day at end is open", &quietHours{start: "09:00", end: "17:00"}, "17:00", false},
		{"overnight inside (late)", &quietHours{start: "22:00", end: "07:00"}, "23:30", true},
		{"overnight inside (early)", &quietHours{start: "22:00", end: "07:00"}, "06:00", true},
		{"overnight outside", &quietHours{start: "22:00", end: "07:00"}, "12:00", false},
		{"malformed is never quiet", &quietHours{start: "", end: "07:00"}, "23:00", false},
	}
	for _, c := range cases {
		p := prefs{quiet: c.q}
		if got := p.quietNow(at(c.now)); got != c.wantQuiet {
			t.Errorf("%s: quietNow(%s)=%v want %v", c.name, c.now, got, c.wantQuiet)
		}
	}
}
