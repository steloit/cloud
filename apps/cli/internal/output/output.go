// Package output is the T5.5 shared rendering layer (cli.md §2): human
// tables with the six status marks, --json verbatim API shapes, --quiet ids.
// Color maps semantic tokens and is NEVER the sole carrier — the mark and
// the word are always present; NO_COLOR and non-TTY degrade losslessly.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// The six marks — spec §1.2 vocabulary, identical on every surface.
var marks = map[string]string{
	"ready":        "✓",
	"provisioning": "◌",
	"degraded":     "!",
	"failed":       "✕",
	"suspended":    "○",
	"deleting":     "·",
}

// semantic color tokens (ok/warn/err/prov) — never the sole carrier.
var colors = map[string]string{
	"ready":        "\033[32m", // ok
	"provisioning": "\033[36m", // prov
	"degraded":     "\033[33m", // warn
	"failed":       "\033[31m", // err
	"suspended":    "\033[2m",
	"deleting":     "\033[2m",
}

const reset = "\033[0m"

// colorEnabled honors NO_COLOR (https://no-color.org) and STELOIT_COLOR=0.
func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("STELOIT_COLOR") == "0" {
		return false
	}
	return os.Getenv("STELOIT_COLOR") == "1" // opt-in until real TTY detection matters
}

// Mark renders `✓ ready` — the mark AND the word, always.
func Mark(status string) string {
	m, ok := marks[status]
	if !ok {
		return status
	}
	if colorEnabled() {
		return colors[status] + m + " " + status + reset
	}
	return m + " " + status
}

// Money renders integer cents with the one arithmetic everywhere: `$58/mo`.
func Money(cents int64) string {
	if cents%100 == 0 {
		return fmt.Sprintf("$%d", cents/100)
	}
	return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}

// MoneyMonthly is Money + the /mo suffix used across every surface.
func MoneyMonthly(cents int64) string { return Money(cents) + "/mo" }

// Table renders aligned columns — one record per row, ids and money in plain
// copyable text (no borders, no truncation).
type Table struct {
	headers []string
	rows    [][]string
}

func NewTable(headers ...string) *Table { return &Table{headers: headers} }

func (t *Table) Row(cells ...string) { t.rows = append(t.rows, cells) }

func (t *Table) Write(w io.Writer) {
	width := make([]int, len(t.headers))
	for i, h := range t.headers {
		width[i] = utf8.RuneCountInString(h)
	}
	for _, r := range t.rows {
		for i, c := range r {
			if i < len(width) && utf8.RuneCountInString(stripANSI(c)) > width[i] {
				width[i] = utf8.RuneCountInString(stripANSI(c))
			}
		}
	}
	line := func(cells []string) {
		parts := make([]string, len(cells))
		for i, c := range cells {
			pad := width[i] - utf8.RuneCountInString(stripANSI(c))
			if pad < 0 {
				pad = 0
			}
			parts[i] = c + strings.Repeat(" ", pad)
		}
		fmt.Fprintln(w, strings.TrimRight(strings.Join(parts, "  "), " "))
	}
	line(t.headers)
	for _, r := range t.rows {
		line(r)
	}
}

func stripANSI(s string) string {
	for {
		i := strings.Index(s, "\033[")
		if i < 0 {
			return s
		}
		j := strings.IndexByte(s[i:], 'm')
		if j < 0 {
			return s
		}
		s = s[:i] + s[i+j+1:]
	}
}

// JSON writes the raw API response shape VERBATIM (snake_case, *_cents,
// {data, next_cursor}) — scripts parse this, never the human tables. v must
// be the decoded API body, not a CLI-invented shape.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// RawJSON re-emits API bytes untouched — the strongest form of "verbatim".
func RawJSON(w io.Writer, body []byte) {
	trimmed := strings.TrimSpace(string(body))
	fmt.Fprintln(w, trimmed)
}

// Quiet writes ids only, one per line (pipe fodder).
func Quiet(w io.Writer, ids ...string) {
	for _, id := range ids {
		fmt.Fprintln(w, id)
	}
}

// Problem renders problem+json as the three-line contract (cli.md §4):
// what happened · why/where · what to do next. Returns the §4 exit code.
type ProblemBody struct {
	Title       string   `json:"title"`
	Detail      string   `json:"detail"`
	Status      int      `json:"status"`
	DeniedBy    string   `json:"denied_by"`
	Reasons     []string `json:"reasons"`
	Remediation string   `json:"remediation"`
	RetryAfterS int      `json:"retry_after_s"`
}

func Problem(w io.Writer, raw []byte, httpStatus int) int {
	var p ProblemBody
	_ = json.Unmarshal(raw, &p)
	if p.Status == 0 {
		p.Status = httpStatus
	}
	what := p.Title
	if p.Detail != "" {
		what += " — " + p.Detail
	}
	if what == "" {
		what = fmt.Sprintf("request failed (%d)", p.Status)
	}
	fmt.Fprintf(w, "✕ %s\n", what)
	switch {
	case p.DeniedBy != "":
		fmt.Fprintf(w, "  %s\n", p.DeniedBy)
	case len(p.Reasons) > 0:
		for _, r := range p.Reasons {
			fmt.Fprintf(w, "  · %s\n", r)
		}
	case p.RetryAfterS > 0:
		fmt.Fprintf(w, "  rate limited — retry after %ds\n", p.RetryAfterS)
	}
	if p.Remediation != "" {
		fmt.Fprintf(w, "  → %s\n", p.Remediation)
	}
	switch p.Status {
	case 401, 403:
		return 3
	case 404:
		return 4
	case 409:
		return 5
	case 402:
		return 6
	case 429:
		return 7
	default:
		return 1
	}
}
