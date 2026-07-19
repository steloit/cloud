package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestTailResumesFromCursor: the fake serves two frames then closes; the
// reconnect must carry ?cursor=<last id> and stops after maxReconnects.
func TestTailResumesFromCursor(t *testing.T) {
	isolate(t)
	var cursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/events") || r.Header.Get("Accept") != "text/event-stream" {
			http.NotFound(w, r)
			return
		}
		cursors = append(cursors, r.URL.Query().Get("cursor"))
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		n := len(cursors)
		fmt.Fprintf(w, "id: cur_%d_1\nevent: lifecycle\ndata: {\"kind\":\"lifecycle\",\"action\":\"a%d\",\"actor\":\"u\",\"at\":\"2026-07-19T10:00:00Z\"}\n\n", n, n)
		fmt.Fprintf(w, ": ping\n\n")
		fmt.Fprintf(w, "id: cur_%d_2\nevent: deploy\ndata: {\"kind\":\"deploy\",\"action\":\"b%d\",\"actor\":\"u\",\"at\":\"2026-07-19T10:00:01Z\"}\n\n", n, n)
	}))
	t.Cleanup(srv.Close)

	cfg := &Config{APIURL: srv.URL, Token: "stp_t"}
	inv := &Invocation{Config: cfg, Flags: map[string]string{}, Stdout: &strings.Builder{}, Stderr: &strings.Builder{}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var got []string
	err := inv.tailEvents(ctx, "env_1", "", 1, func(f sseFrame) error {
		got = append(got, f.Event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// two connections: initial (no cursor) + one reconnect carrying the last id
	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "cur_1_2" {
		t.Fatalf("cursors: %v", cursors)
	}
	// four frames total; heartbeats never surface
	if len(got) != 4 || got[0] != "lifecycle" || got[1] != "deploy" {
		t.Fatalf("frames: %v", got)
	}
}

func TestReadSSEParsesHeartbeatsAndFrames(t *testing.T) {
	stream := "id: c1\nevent: lifecycle\ndata: {\"a\":1}\n\n: ping\n\nid: c2\ndata: {\"a\":2}\n\n"
	var frames []sseFrame
	err := readSSE(context.Background(), strings.NewReader(stream), func(f sseFrame) error {
		frames = append(frames, f)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 || frames[0].ID != "c1" || frames[0].Event != "lifecycle" || frames[1].ID != "c2" {
		t.Fatalf("frames: %+v", frames)
	}
}
