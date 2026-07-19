package session

import (
	"net/http"
	"testing"
	"time"
)

func TestTokenAndCookie(t *testing.T) {
	m := NewManager(time.Hour, true)
	raw, hash, err := m.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(hash) != 32 {
		t.Fatalf("hash len %d", len(hash))
	}
	raw2, _, _ := m.NewToken()
	if raw == raw2 {
		t.Fatal("tokens not unique")
	}
	ck := m.Cookie(raw, time.Now().Add(time.Hour))
	if !ck.HttpOnly || !ck.Secure || ck.SameSite != http.SameSiteLaxMode || ck.Path != "/" {
		t.Fatalf("cookie attributes wrong: %+v", ck)
	}
	clear := m.ClearCookie()
	if clear.MaxAge != -1 {
		t.Fatal("clear cookie must expire immediately")
	}
}
