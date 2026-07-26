package mailer

// The Resend provider (ADR-0009: primary email provider). A thin HTTP adapter —
// Resend is an implementation behind Provider, not a platform dependency. The
// wire call is exercised end-to-end only once RESEND_API_KEY is provided; until
// then the composition root selects Noop, so this adapter is structurally
// complete but its live path is finalized with the key.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const resendEndpoint = "https://api.resend.com/emails"

// Resend sends via api.resend.com. The API key is an envelope-encrypted platform
// secret, never in code (ADR-0009).
type Resend struct {
	apiKey string
	client *http.Client
}

func NewResend(apiKey string) *Resend {
	return &Resend{apiKey: apiKey, client: &http.Client{Timeout: 15 * time.Second}}
}

func (*Resend) Name() string { return "resend" }

func (r *Resend) Send(ctx context.Context, m Message) (string, error) {
	body, err := json.Marshal(map[string]any{
		"from":    m.From,
		"to":      []string{m.To},
		"subject": m.Subject,
		"html":    m.HTML,
		"text":    m.Text,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("resend: %w", err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("resend: status %d: %s", resp.StatusCode, payload)
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(payload, &out)
	return out.ID, nil
}
