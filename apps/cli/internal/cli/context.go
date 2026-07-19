package cli

// Context resolution (cli.md §1): explicit flags → repo link (`steloit init`
// wrote .steloit) → profile default. The resolved context is ALWAYS echoed
// on state-changing commands — context is worn, not guessed.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Context is the console crumb as data.
type Context struct {
	Org, Project, Env string
	Source            string // "flags" | "repo" | "profile" | "none" (mixed → most explicit wins per field)
}

// RepoLink is the .steloit file `steloit init` writes at the repo root.
type RepoLink struct {
	Org     string `json:"org,omitempty"`
	Project string `json:"project"`
	Env     string `json:"env,omitempty"`
}

// findRepoLink walks upward from dir looking for .steloit.
func findRepoLink(dir string) (*RepoLink, string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, ""
	}
	for {
		p := filepath.Join(abs, ".steloit")
		if raw, err := os.ReadFile(p); err == nil {
			var link RepoLink
			if json.Unmarshal(raw, &link) == nil && link.Project != "" {
				return &link, p
			}
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return nil, ""
		}
		abs = parent
	}
}

// ResolveContext applies the ladder per field: a flag always wins; the repo
// link fills what flags left empty; the profile default fills the rest.
func ResolveContext(flags map[string]string, dir string, cfg *Config) Context {
	c := Context{Org: flags["org"], Project: flags["project"], Env: flags["env"]}
	source := "none"
	if c.Org != "" || c.Project != "" || c.Env != "" {
		source = "flags"
	}
	if link, _ := findRepoLink(dir); link != nil {
		filled := false
		if c.Project == "" && link.Project != "" {
			c.Project, filled = link.Project, true
		}
		if c.Org == "" && link.Org != "" {
			c.Org, filled = link.Org, true
		}
		if c.Env == "" && link.Env != "" {
			c.Env, filled = link.Env, true
		}
		if filled && source == "none" {
			source = "repo"
		}
	}
	if cfg != nil {
		filled := false
		if c.Org == "" && cfg.DefaultOrg != "" {
			c.Org, filled = cfg.DefaultOrg, true
		}
		if c.Project == "" && cfg.DefaultPrj != "" {
			c.Project, filled = cfg.DefaultPrj, true
		}
		if c.Env == "" && cfg.DefaultEnv != "" {
			c.Env, filled = cfg.DefaultEnv, true
		}
		if filled && source == "none" {
			source = "profile"
		}
	}
	c.Source = source
	return c
}

// Echo prints the worn context (`ecommerce/production ·`) before
// state-changing output. Env omitted = production forever (ADR-037) — the
// echo says so rather than guessing.
func (c Context) Echo() string {
	prj := c.Project
	if prj == "" {
		return ""
	}
	env := c.Env
	if env == "" {
		env = "production"
	}
	return fmt.Sprintf("%s/%s ·", prj, env)
}
