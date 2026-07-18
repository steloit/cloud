package ratelimit

import (
	"testing"
	"time"
)

func TestWindowLimit(t *testing.T) {
	l := New(3, 50*time.Millisecond)
	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("k"); !ok {
			t.Fatalf("call %d denied", i)
		}
	}
	ok, retry := l.Allow("k")
	if ok || retry < 1 {
		t.Fatalf("4th call allowed or retry=%d", retry)
	}
	if ok, _ := l.Allow("other"); !ok {
		t.Fatal("independent key denied")
	}
	time.Sleep(60 * time.Millisecond)
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("window did not reset")
	}
}
