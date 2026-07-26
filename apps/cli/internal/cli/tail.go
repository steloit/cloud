package cli

// T5.4: live tail (-f) over SSE — the spine drives CLI -f (T2.5's stream).
// Reconnects resume from the last frame's id (the cursor): no gap, no
// duplicate — the same contract the console's SSE client honors.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// sseFrame is one server-sent event.
type sseFrame struct {
	ID, Event, Data string
}

// readSSE parses frames from a stream, invoking fn per frame; returns on
// stream end or ctx cancel.
func readSSE(ctx context.Context, r io.Reader, fn func(sseFrame) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	var f sseFrame
	for sc.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "id: "):
			f.ID = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: "):
			f.Event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			f.Data = strings.TrimPrefix(line, "data: ")
		case strings.HasPrefix(line, ":"):
			// heartbeat — keep waiting
		case line == "":
			if f.Data != "" {
				if err := fn(f); err != nil {
					return err
				}
			}
			f = sseFrame{}
		}
	}
	return sc.Err()
}

// tailEvents streams /envs/{env}/events with Accept: text/event-stream,
// resuming from the last cursor on reconnect (up to maxReconnects; -1 =
// forever). Rendering goes through fn.
func (inv *Invocation) tailEvents(ctx context.Context, envID string, kind string, maxReconnects int, fn func(sseFrame) error) error {
	cursor := ""
	attempts := 0
	client := &http.Client{} // no timeout: this is a long-lived stream
	for {
		url := strings.TrimSuffix(inv.Config.APIURL, "/") + "/v1/envs/" + envID + "/events"
		sep := "?"
		if kind != "" {
			url += sep + "kind=" + kind
			sep = "&"
		}
		if cursor != "" {
			url += sep + "cursor=" + cursor
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Authorization", "Bearer "+inv.Config.Token)
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
			resp.Body.Close()
			return fmt.Errorf("stream: %s", strings.TrimSpace(string(body)))
		}
		err = readSSE(ctx, resp.Body, func(f sseFrame) error {
			if f.ID != "" {
				cursor = f.ID // resume point survives reconnects
			}
			return fn(f)
		})
		resp.Body.Close()
		if ctx.Err() != nil || err == io.EOF {
			return nil
		}
		if err != nil && err != context.Canceled {
			return err
		}
		// stream ended (server restart, LB timeout): resume from the cursor
		attempts++
		if maxReconnects >= 0 && attempts > maxReconnects {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
		}
	}
}

// renderEventFrame prints one spine event as a tail line.
func renderEventFrame(w io.Writer, f sseFrame) {
	var e struct {
		At     time.Time `json:"at"`
		Kind   string    `json:"kind"`
		Action string    `json:"action"`
		Actor  string    `json:"actor"`
	}
	if json.Unmarshal([]byte(f.Data), &e) != nil {
		fmt.Fprintln(w, f.Data)
		return
	}
	fmt.Fprintf(w, "%s  %-12s %-24s %s\n", e.At.Format("15:04:05"), e.Kind, e.Action, e.Actor)
}
