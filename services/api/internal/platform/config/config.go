// Package config is the one typed configuration struct (architecture §16/§17):
// 12-factor env vars, validated at boot, fail fast, effective non-secret
// values printed once.
package config

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Port         string        `env:"PORT" envDefault:"8080"`
	DatabaseURL  string        `env:"DATABASE_URL,required"`
	SessionTTL   time.Duration `env:"SESSION_TTL" envDefault:"720h"` // 30d absolute
	CookieSecure bool          `env:"COOKIE_SECURE" envDefault:"true"`

	// Argon2id parameters — OWASP-recommended defaults; tune only with
	// measured evidence.
	// SecretsKEK is the base64 32-byte key-encryption key (D5 envelope);
	// GCP KMS replaces the env source behind the same seam at P1.
	SecretsKEK   string `env:"SECRETS_KEK,required"`
	SecretsKEKID string `env:"SECRETS_KEK_ID" envDefault:"env-v1"`

	// GithubWebhookSecret guards /integrations/github/webhook; empty = the
	// app isn't registered yet (P2) and the ingress refuses with 503.
	GithubWebhookSecret string `env:"GITHUB_WEBHOOK_SECRET"`

	// ReconcilerSecret guards the internal reconcile plane (US-1.3). Empty =
	// no cell-agent is enrolled yet and the endpoints answer 503 — the
	// fail-closed shape; absent config must never mean open.
	ReconcilerSecret string `env:"RECONCILER_SECRET"`
	// ReconcilerCells is the comma-separated cell list that secret may act on.
	ReconcilerCells []string `env:"RECONCILER_CELLS" envSeparator:"," envDefault:"cell-0"`

	ArgonMemoryKiB uint32 `env:"ARGON_MEMORY_KIB" envDefault:"19456"`
	ArgonTime      uint32 `env:"ARGON_TIME" envDefault:"2"`
	ArgonThreads   uint8  `env:"ARGON_THREADS" envDefault:"1"`

	// Email (T10.4, ADR-0009). Empty RESEND_API_KEY ⇒ the Noop provider (the
	// app runs without a live send path; the key finalizes it — GCP Secret
	// Manager replaces the env source behind the same seam later).
	ResendAPIKey   string `env:"RESEND_API_KEY"`
	EmailFrom      string `env:"EMAIL_FROM" envDefault:"Steloit <noreply@steloit.app>"`
	ConsoleBaseURL string `env:"CONSOLE_BASE_URL" envDefault:"https://console.steloit.app"`
}

func Load() (Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) Validate() error {
	if c.SessionTTL < time.Minute {
		return fmt.Errorf("config: SESSION_TTL %s is below the 1m floor", c.SessionTTL)
	}
	if c.ArgonMemoryKiB < 8*1024 {
		return fmt.Errorf("config: ARGON_MEMORY_KIB %d is below the 8MiB floor", c.ArgonMemoryKiB)
	}
	return nil
}

// LogEffective prints the non-secret effective config once at boot (§16).
func (c Config) LogEffective(l *slog.Logger) {
	l.Info("effective config",
		"port", c.Port,
		"session_ttl", c.SessionTTL.String(),
		"cookie_secure", c.CookieSecure,
		"argon_memory_kib", c.ArgonMemoryKiB,
		"argon_time", c.ArgonTime,
		"argon_threads", c.ArgonThreads,
	)
}
