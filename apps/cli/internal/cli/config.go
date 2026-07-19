package cli

// Profile config: ~/.config/steloit/config.json (XDG-style). Holds the API
// base URL, the bearer token (written by `steloit auth login`, T5.2), and
// profile-default context — the LAST rung of the resolution ladder.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type Config struct {
	APIURL     string `json:"api_url,omitempty"`
	Token      string `json:"token,omitempty"` // stp_… — stored with 0600, never echoed
	DefaultOrg string `json:"default_org,omitempty"`
	DefaultPrj string `json:"default_project,omitempty"`
	DefaultEnv string `json:"default_env,omitempty"`

	path string
}

// configPath honors STELOIT_CONFIG for tests, XDG_CONFIG_HOME, then HOME.
func configPath() (string, error) {
	if p := os.Getenv("STELOIT_CONFIG"); p != "" {
		return p, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "steloit", "config.json"), nil
}

func LoadConfig() (*Config, error) {
	p, err := configPath()
	if err != nil {
		return nil, err
	}
	cfg := &Config{path: p, APIURL: "https://api.steloit.com"}
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", p, err)
	}
	if cfg.APIURL == "" {
		cfg.APIURL = "https://api.steloit.com"
	}
	if env := os.Getenv("STELOIT_API_URL"); env != "" {
		cfg.APIURL = env
	}
	return cfg, nil
}

// Save writes 0600 — the file may hold a token.
func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, append(raw, '\n'), 0o600)
}
