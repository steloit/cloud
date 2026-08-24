package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// HTTPControlPlane talks to the control plane's internal reconcile endpoints
// (US-1.3). It carries the reconciler-scoped bearer and nothing else — this
// client is never a user principal.
type HTTPControlPlane struct {
	base  string // e.g. https://api.internal.steloit.com
	token string
	hc    *http.Client
}

func NewHTTPControlPlane(base, token string) *HTTPControlPlane {
	return &HTTPControlPlane{
		base:  base,
		token: token,
		// A bounded timeout is part of the outage guarantee: a hung control
		// plane must surface as a poll error the loop skips, never a wedged tick.
		hc: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *HTTPControlPlane) Desired(ctx context.Context, cell string, since int64) (DesiredState, error) {
	u := fmt.Sprintf("%s/v1/reconcile/%s/desired?since_generation=%s",
		c.base, url.PathEscape(cell), strconv.FormatInt(since, 10))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return DesiredState{}, err
	}
	c.auth(req)
	resp, err := c.hc.Do(req)
	if err != nil {
		return DesiredState{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return DesiredState{}, statusErr("desired", resp)
	}
	var body DesiredState
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return DesiredState{}, fmt.Errorf("desired: decode: %w", err)
	}
	return body, nil
}

// ConfirmEnvironmentTeardown reports that an environment's namespace is gone.
//
// A 409 here means the environment is NOT awaiting a teardown — never scheduled,
// or already confirmed — and unlike the status route's 409s it is not a
// "re-poll". The loop logs and moves on; the environment has already stopped
// being advertised, so there is nothing to retry.
func (c *HTTPControlPlane) ConfirmEnvironmentTeardown(ctx context.Context, cell, envID string) error {
	buf, err := json.Marshal(map[string]string{"observed": "gone"})
	if err != nil {
		return err
	}
	u := fmt.Sprintf("%s/v1/reconcile/%s/environments/%s/teardown",
		c.base, url.PathEscape(cell), url.PathEscape(envID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.auth(req)
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return statusErr("environment teardown", resp)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *HTTPControlPlane) Report(ctx context.Context, cell string, r Report) error {
	buf, err := json.Marshal(r)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("%s/v1/reconcile/%s/status", c.base, cell)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.auth(req)
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return statusErr("status", resp)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *HTTPControlPlane) auth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
}

// statusErr renders a non-200 as an error carrying the code, so the loop can log
// it and skip. A 409 (generation mismatch) is just another "re-poll" — the row
// stays outstanding server-side, so the next tick re-converges it; no special
// case is needed.
func statusErr(op string, resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	return fmt.Errorf("%s: control plane returned %d: %s", op, resp.StatusCode, bytes.TrimSpace(b))
}
