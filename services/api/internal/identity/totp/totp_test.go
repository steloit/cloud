package totp

import (
	"testing"
	"time"
)

func TestCodeAndVerify(t *testing.T) {
	sec, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	code, err := Code(sec, now)
	if err != nil || len(code) != 6 {
		t.Fatalf("code: %q %v", code, err)
	}
	if !Verify(sec, code, now) {
		t.Fatal("fresh code did not verify")
	}
	if !Verify(sec, code, now.Add(29*time.Second)) {
		t.Fatal("in-window skew rejected")
	}
	if Verify(sec, code, now.Add(120*time.Second)) {
		t.Fatal("out-of-window code accepted")
	}
	if Verify(sec, "12345", now) || Verify(sec, "abcdef", now) {
		t.Fatal("malformed code accepted")
	}
}

// RFC 6238 Appendix B vector (SHA-1, ASCII secret "12345678901234567890").
func TestRFC6238Vector(t *testing.T) {
	sec := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	got, err := Code(sec, time.Unix(59, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got != "287082" {
		t.Fatalf("RFC vector T=59: got %s want 287082", got)
	}
}
