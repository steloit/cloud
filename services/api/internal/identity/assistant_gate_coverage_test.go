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

// assistantOpsFromSpec returns every operationId under a `tags: [assistant]`
// block in the openapi contract.
func assistantOpsFromSpec(t *testing.T) map[string]bool {
	t.Helper()
	spec, err := os.ReadFile("../../../../docs/product/08-api/openapi.yaml")
	if err != nil {
		t.Fatalf("read openapi: %v", err)
	}
	ops := map[string]bool{}
	// scan each operationId and look at a small window for the assistant tag.
	re := regexp.MustCompile(`operationId:\s*(\w+)`)
	s := string(spec)
	for _, m := range re.FindAllStringSubmatchIndex(s, -1) {
		op := s[m[2]:m[3]]
		lo := m[0] - 160
		if lo < 0 {
			lo = 0
		}
		hi := m[1] + 160
		if hi > len(s) {
			hi = len(s)
		}
		if strings.Contains(s[lo:hi], "[assistant]") {
			ops[op] = true
		}
	}
	if len(ops) == 0 {
		t.Fatal("no assistant-tagged operations found — spec path or parse broke")
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

// TestEveryAssistantHandlerGatesOnPolicy: every IMPLEMENTED assistant operation
// must have a handler whose body enforces AIAssistantEnabled (the Law-4 gate).
func TestEveryAssistantHandlerGatesOnPolicy(t *testing.T) {
	assistant := assistantOpsFromSpec(t)
	implemented := implementedOps(t)

	// gather handler source across the modules that can host assistant handlers.
	bodies := map[string]string{} // MethodName → function body
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
			src := string(b)
			fn := regexp.MustCompile(`func \(h \*Handlers\) (\w+)\(`)
			locs := fn.FindAllStringSubmatchIndex(src, -1)
			for i, m := range locs {
				name := src[m[2]:m[3]]
				end := len(src)
				if i+1 < len(locs) {
					end = locs[i+1][0]
				}
				bodies[name] = src[m[1]:end]
			}
		}
	}

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
		if !strings.Contains(body, "AIAssistantEnabled") {
			t.Errorf("assistant handler %s does not enforce AIAssistantEnabled — AI Law 4 requires every /assistant/* surface to 404 when the policy is disabled", pascal(op))
		}
	}
	if covered == 0 {
		t.Fatal("no implemented assistant operations were checked — the tripwire is inert (did the include list or spec parse break?)")
	}
	t.Logf("verified %d implemented assistant operation(s) gate on AIAssistantEnabled", covered)
}
