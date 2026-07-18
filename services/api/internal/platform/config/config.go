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

	ArgonMemoryKiB uint32 `env:"ARGON_MEMORY_KIB" envDefault:"19456"`
	ArgonTime      uint32 `env:"ARGON_TIME" envDefault:"2"`
	ArgonThreads   uint8  `env:"ARGON_THREADS" envDefault:"1"`
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
