package output

import (
	"bytes"
	"strings"
	"testing"
)

// AC: six status marks; color never the sole carrier.
func TestSixMarks(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	want := map[string]string{
		"ready":        "✓ ready",
		"provisioning": "◌ provisioning",
		"degraded":     "! degraded",
		"failed":       "✕ failed",
		"suspended":    "○ suspended",
		"deleting":     "· deleting",
	}
	for status, expect := range want {
		if got := Mark(status); got != expect {
			t.Fatalf("%s: %q", status, got)
		}
	}
	// with color ON, the mark and the word are STILL present (color never
	// the sole carrier), and NO_COLOR strips escapes entirely
	t.Setenv("NO_COLOR", "")
	t.Setenv("STELOIT_COLOR", "1")
	got := Mark("failed")
	if !strings.Contains(got, "✕") || !strings.Contains(got, "failed") {
		t.Fatalf("color dropped the mark or word: %q", got)
	}
	t.Setenv("NO_COLOR", "x")
	if got := Mark("failed"); strings.Contains(got, "\033") {
		t.Fatalf("NO_COLOR not honored: %q", got)
	}
	// unknown statuses degrade to the word, never panic
	if got := Mark("running"); got != "running" {
		t.Fatalf("unknown status: %q", got)
	}
}

func TestMoney(t *testing.T) {
	if Money(5800) != "$58" || MoneyMonthly(5800) != "$58/mo" {
		t.Fatalf("whole: %s", MoneyMonthly(5800))
	}
	if Money(162) != "$1.62" {
		t.Fatalf("fractional: %s", Money(162))
	}
	if Money(20800) != "$208" {
		t.Fatalf("canon: %s", Money(20800))
	}
}

func TestTableAlignment(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	tab := NewTable("NAME", "STATUS", "COST")
	tab.Row("db-main", Mark("degraded"), MoneyMonthly(5800))
	tab.Row("api", Mark("ready"), MoneyMonthly(6100))
	var buf bytes.Buffer
	tab.Write(&buf)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines: %q", buf.String())
	}
	// aligned: STATUS starts at the same rune column on every row
	col := strings.Index(lines[0], "STATUS")
	for _, l := range lines[1:] {
		if l[col] == ' ' {
			t.Fatalf("misaligned: %q", buf.String())
		}
	}
	if !strings.Contains(buf.String(), "$58/mo") {
		t.Fatalf("money missing: %q", buf.String())
	}
}

func TestProblemThreeLines(t *testing.T) {
	var buf bytes.Buffer
	// 403: the E3 grammar lives in DETAIL and becomes the why/where line
	code := Problem(&buf, []byte(`{"title":"Access denied","detail":"role:developer lacks org.manage","status":403,"remediation":"Ask an org admin."}`), 403, "")
	out := buf.String()
	if code != 3 {
		t.Fatalf("exit: %d", code)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 ||
		!strings.Contains(lines[0], "Access denied") ||
		!strings.Contains(lines[1], "role:developer") ||
		!strings.Contains(lines[2], "Ask an org admin") {
		t.Fatalf("three lines: %q", out)
	}
	// 409: contract OBJECT reasons, all listed with their remediations; exit 5
	buf.Reset()
	code = Problem(&buf, []byte(`{"title":"Conflict","status":409,"reasons":[{"code":"c1","message":"a","remediation":"do a"},{"message":"b"}],"remediation":"fix"}`), 409, "")
	if code != 5 || strings.Count(buf.String(), "·") != 2 || !strings.Contains(buf.String(), "do a") {
		t.Fatalf("409 objects: %d %q", code, buf.String())
	}
	// bare-string reasons tolerated during rollout
	buf.Reset()
	code = Problem(&buf, []byte(`{"title":"Conflict","status":409,"reasons":["x","y"],"remediation":"fix"}`), 409, "")
	if code != 5 || strings.Count(buf.String(), "·") != 2 {
		t.Fatalf("409 strings: %d %q", code, buf.String())
	}
	// 402 exit 6 · 429 exit 7 with the HEADER carrying timing · garbage degrades
	if code = Problem(&bytes.Buffer{}, []byte(`{"status":402}`), 402, ""); code != 6 {
		t.Fatalf("402: %d", code)
	}
	buf.Reset()
	if code = Problem(&buf, []byte(`{"status":429,"title":"Rate limited"}`), 429, "30"); code != 7 || !strings.Contains(buf.String(), "30s") {
		t.Fatalf("429: %d %q", code, buf.String())
	}
	if code = Problem(&bytes.Buffer{}, []byte(`not json`), 500, ""); code != 1 {
		t.Fatalf("garbage: %d", code)
	}
}

func TestQuietAndRawJSON(t *testing.T) {
	var buf bytes.Buffer
	Quiet(&buf, "svc_a", "svc_b")
	if buf.String() != "svc_a\nsvc_b\n" {
		t.Fatalf("quiet: %q", buf.String())
	}
	buf.Reset()
	RawJSON(&buf, []byte("  {\"monthly_total_cents\":20800}\n"))
	if buf.String() != "{\"monthly_total_cents\":20800}\n" {
		t.Fatalf("raw: %q", buf.String())
	}
}
