package identity_test

// T12.3 — the ai-assistant policy (enabled | opt_in | disabled) as a STRUCTURAL
// guarantee: AI Law 4 says disabling hides EVERY /assistant/* surface (404
// empty-equivalent), deletes nothing, re-enables instantly. T13.3 landed the
// enforcement primitive (identity.Service.AIAssistantEnabled) and the first
// consumer (threads); this tripwire makes "gate ALL /assistant/*" enforced by
// construction — a new assistant endpoint that forgets the gate FAILS here,
// rather than silently shipping an AI surface that ignores the org policy.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// assistantOpsFromSpec returns every operationId reachable under a `/assistant/`
// PATH block in the openapi contract. Keying off the path — not a `tags:` window
// — is both the actual Law-4 surface (the /assistant/* URLs) and robust to tag
// formatting (inline `[assistant]`, multi-tag `[assistant, ai]`, or block YAML
// all parse the same, because tags aren't consulted). Review-hardened: the old
// ±160-char tag window could false-negative a multi-tag op and silently drop it
// from the checked set, letting an ungated handler ship green.
func assistantOpsFromSpec(t *testing.T) map[string]bool {
	t.Helper()
	spec, err := os.ReadFile("../../../../docs/product/08-api/openapi.yaml")
	if err != nil {
		t.Fatalf("read openapi: %v", err)
	}
	ops := opsUnderAssistantPaths(string(spec))
	if len(ops) == 0 {
		t.Fatal("no /assistant/ operations found — spec path or parse broke")
	}
	return ops
}

// opsUnderAssistantPaths is the pure parser (unit-testable without the file):
// under `paths:`, a path key sits at 2-space indent (`  /assistant/threads:`);
// its operations run until the next 2-space path key. Collect every
// operationId inside a path whose key begins with `/assistant/`.
func opsUnderAssistantPaths(spec string) map[string]bool {
	ops := map[string]bool{}
	pathKey := regexp.MustCompile(`^  (/\S*):\s*$`)
	opID := regexp.MustCompile(`^\s+operationId:\s*(\w+)`)
	inAssistant := false
	for _, line := range strings.Split(spec, "\n") {
		if m := pathKey.FindStringSubmatch(line); m != nil {
			inAssistant = strings.HasPrefix(m[1], "/assistant/")
			continue
		}
		if inAssistant {
			if m := opID.FindStringSubmatch(line); m != nil {
				ops[m[1]] = true
			}
		}
	}
	return ops
}

// implementedOps returns the strict server's include-operation-ids set.
func implementedOps(t *testing.T) map[string]bool {
	t.Helper()
	cfg, err := os.ReadFile("../../oapi-server.cfg.yaml")
	if err != nil {
		t.Fatalf("read oapi-server.cfg: %v", err)
	}
	m := regexp.MustCompile(`include-operation-ids:\s*\[([^\]]*)\]`).FindSubmatch(cfg)
	if m == nil {
		t.Fatal("include-operation-ids not found")
	}
	out := map[string]bool{}
	for _, id := range strings.Split(string(m[1]), ",") {
		if id = strings.TrimSpace(id); id != "" {
			out[id] = true
		}
	}
	return out
}

// pascal upper-cases the first rune (oapi-codegen derives the handler method
// name from the operationId this way: createThread → CreateThread).
func pascal(op string) string {
	if op == "" {
		return op
	}
	return strings.ToUpper(op[:1]) + op[1:]
}

// handlerBodies extracts each `(h *Handlers) Name` method body from Go source,
// bounded by BRACE MATCHING (review-hardened: the old next-func/EOF bound let a
// trailing package-level helper mentioning AIAssistantEnabled be miscredited to
// the last handler in a file).
func handlerBodies(src string) map[string]string {
	out := map[string]string{}
	fn := regexp.MustCompile(`func \(h \*Handlers\) (\w+)\([^)]*\)`)
	for _, m := range fn.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		// find the opening brace of the body after the signature/return types.
		open := strings.IndexByte(src[m[1]:], '{')
		if open < 0 {
			continue
		}
		i := m[1] + open
		depth := 0
		for j := i; j < len(src); j++ {
			switch src[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					out[name] = src[i : j+1]
					goto next
				}
			}
		}
	next:
	}
	return out
}

// gatesOnAIAssistant is the enforcement predicate, factored out so its RED state
// is directly testable (review: a tripwire whose failure branch never runs can
// silently rot to always-green).
func gatesOnAIAssistant(body string) bool {
	return strings.Contains(body, "AIAssistantEnabled")
}

func collectHandlerBodies(t *testing.T) map[string]string {
	t.Helper()
	bodies := map[string]string{}
	for _, dir := range []string{".", "../provisioning"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			for name, body := range handlerBodies(string(b)) {
				bodies[name] = body
			}
		}
	}
	return bodies
}

// TestEveryAssistantHandlerGatesOnPolicy: every IMPLEMENTED assistant operation
// must have a handler whose body enforces AIAssistantEnabled (the Law-4 gate).
func TestEveryAssistantHandlerGatesOnPolicy(t *testing.T) {
	assistant := assistantOpsFromSpec(t)
	implemented := implementedOps(t)
	bodies := collectHandlerBodies(t)

	var covered int
	for op := range assistant {
		if !implemented[op] {
			continue // not shipped yet — the gate lands with the endpoint
		}
		covered++
		body, ok := bodies[pascal(op)]
		if !ok {
			t.Errorf("assistant op %q is in the include list but no (h *Handlers) %s handler was found", op, pascal(op))
			continue
		}
		if !gatesOnAIAssistant(body) {
			t.Errorf("assistant handler %s does not enforce AIAssistantEnabled — AI Law 4 requires every /assistant/* surface to 404 when the policy is disabled", pascal(op))
		}
	}
	if covered == 0 {
		t.Fatal("no implemented assistant operations were checked — the tripwire is inert (did the include list or spec parse break?)")
	}
	t.Logf("verified %d implemented assistant operation(s) gate on AIAssistantEnabled", covered)
}

// TestGateDetectorHasARedState proves the tripwire's detector actually FAILS on
// an ungated handler and PASSES a gated one — without this, a regression that
// broke the detector (bad regex, inverted Contains, over-wide body) would leave
// the whole tripwire silently always-green (QA finding, high).
func TestGateDetectorHasARedState(t *testing.T) {
	const src = `package x
func (h *Handlers) Gated(ctx context.Context) error {
	on, _ := h.svc.AIAssistantEnabled(ctx, org, "")
	if !on { return aiDisabled() }
	return nil
}
func (h *Handlers) Ungated(ctx context.Context) error {
	return h.do(ctx)
}
// a trailing package-level helper that mentions AIAssistantEnabled must NOT be
// miscredited to Ungated (the brace-bound extraction proves it).
func requireAIAssistantEnabled() {}
`
	bodies := handlerBodies(src)
	if b, ok := bodies["Gated"]; !ok || !gatesOnAIAssistant(b) {
		t.Fatalf("detector failed to see the gate in Gated (ok=%v)", ok)
	}
	if b, ok := bodies["Ungated"]; !ok || gatesOnAIAssistant(b) {
		t.Fatalf("detector FALSE-PASSED an ungated handler — the trailing helper leaked into its body (ok=%v body=%q)", ok, bodies["Ungated"])
	}
}

// TestAssistantOpsSurviveTagFormats proves the /assistant/-path detector finds
// operations regardless of tag formatting (the old tag-window would drop these).
func TestAssistantOpsSurviveTagFormats(t *testing.T) {
	const spec = `
paths:
  /assistant/threads:
    post:
      tags: [assistant, ai]
      operationId: createThread
    get:
      tags:
        - assistant
      operationId: listThreads
  /orgs/{org}/dashboards:
    post:
      tags: [observe]
      operationId: createDashboard
`
	ops := opsUnderAssistantPaths(spec)
	if !ops["createThread"] || !ops["listThreads"] {
		t.Fatalf("multi-tag / block-tag assistant ops were dropped: %v", ops)
	}
	if ops["createDashboard"] {
		t.Fatalf("a non-assistant path's op leaked into the assistant set: %v", ops)
	}
}
