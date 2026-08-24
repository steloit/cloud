package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func repoRootFromAgent(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("repo root not found (AGENTS.md) — a contract test must not silently disarm")
	return ""
}

func spec(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRootFromAgent(t),
		"docs", "product", "08-api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}
	return doc
}

func dig(t *testing.T, m map[string]any, path ...string) any {
	t.Helper()
	var cur any = m
	for _, k := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("openapi.yaml: %v is not a mapping at %q", path, k)
		}
		cur, ok = mm[k]
		if !ok {
			t.Fatalf("openapi.yaml has no %v (stopped at %q) — the contract moved and this "+
				"test would otherwise pass by looking at nothing", path, k)
		}
	}
	return cur
}

// THE AGENT'S STRUCT TAGS ARE BOUND TO THE CONTRACT.
//
// The two ends of this wire are separate Go modules with nothing between them:
// the control plane writes the poll body from a literal map, the agent decodes
// it into these structs, and no compiler checks that the names agree. Measured
// on the first cut: renaming the agent's tag to `env_teardowns` left the whole
// module green while the feature silently stopped working in production.
//
// So both ends bind to the SPEC, which is the authority they share.
func TestTheAgentDecodesTheKeysTheContractDeclares(t *testing.T) {
	doc := spec(t)
	props := dig(t, doc, "paths", "/reconcile/{cell}/desired", "get", "responses", "200",
		"content", "application/json", "schema", "properties").(map[string]any)
	for _, want := range []string{"services", "environments"} {
		if _, ok := props[want]; !ok {
			t.Fatalf("the contract's /desired response has no %q property", want)
		}
	}
	// Round-trip a body keyed exactly as the contract declares; the agent must
	// see it. A tag rename fails here.
	body := `{"services":[],"environments":[{"id":"env_x","namespace":"env-x"}]}`
	var st DesiredState
	if err := json.Unmarshal([]byte(body), &st); err != nil {
		t.Fatal(err)
	}
	if len(st.Environments) != 1 {
		t.Fatalf("the agent did not decode `environments` from a contract-shaped body: %s", body)
	}
	if st.Environments[0].ID != "env_x" || st.Environments[0].Namespace != "env-x" {
		t.Fatalf("decoded %+v — the id/namespace tags do not match the contract",
			st.Environments[0])
	}

	envProps := dig(t, doc, "components", "schemas", "DesiredEnvironmentTeardown",
		"properties").(map[string]any)
	for _, want := range []string{"id", "namespace"} {
		if _, ok := envProps[want]; !ok {
			t.Fatalf("DesiredEnvironmentTeardown has no %q in the contract", want)
		}
	}
}

// THE CONFIRMATION SPEAKS THE ROUTE THE CONTRACT DECLARES.
//
// Three independent things were unpinned and each is "the feature silently does
// not work while CI is green": the URL, the body, and the non-200 check.
// Measured on the first cut — changing the path, sending `{"observed":"ready"}`,
// and dropping the status check ALL survived the whole module.
//
// The mux mounts ONLY the contract's path, so anything else is a 404 the client
// must surface.
func TestConfirmEnvironmentTeardownSpeaksTheContractRoute(t *testing.T) {
	// The path, from the spec rather than retyped.
	paths := dig(t, spec(t), "paths").(map[string]any)
	const want = "/reconcile/{cell}/environments/{env}/teardown"
	if _, ok := paths[want]; !ok {
		t.Fatalf("the contract has no %q — this test is pinning a route that moved", want)
	}
	enum := dig(t, spec(t), "paths", want, "post", "requestBody", "content", "application/json",
		"schema", "properties", "observed", "enum").([]any)
	if len(enum) != 1 || enum[0] != "gone" {
		t.Fatalf("the contract's `observed` enum is %v, want exactly [gone]", enum)
	}

	var gotPath, gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/reconcile/{cell}/environments/{env}/teardown",
		func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"environment_id":"env_x","torn_down":true}`))
		})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	cp := NewHTTPControlPlane(ts.URL, "s3cret")
	if err := cp.ConfirmEnvironmentTeardown(context.Background(), "cell-0", "env_x"); err != nil {
		t.Fatalf("confirm against the contract's own route failed: %v — the client is not "+
			"speaking the path the contract declares", err)
	}
	if gotPath != "/v1/reconcile/cell-0/environments/env_x/teardown" {
		t.Errorf("posted to %q", gotPath)
	}
	// The body must carry the ONE value the enum admits. Anything else 422s in
	// production, forever, and the namespace is re-deleted every tick.
	var sent map[string]string
	if err := json.Unmarshal([]byte(gotBody), &sent); err != nil {
		t.Fatalf("body %q is not JSON: %v", gotBody, err)
	}
	if sent["observed"] != "gone" {
		t.Errorf("sent observed=%q, want %q — the API refuses anything else with 422",
			sent["observed"], enum[0])
	}
}

// A NON-200 IS AN ERROR. Without this the client returns nil on a 404 or 409 and
// the loop confirms a teardown the control plane refused.
func TestConfirmEnvironmentTeardownSurfacesRefusals(t *testing.T) {
	for _, code := range []int{404, 409, 422, 500} {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"title":"nope"}`))
		}))
		cp := NewHTTPControlPlane(ts.URL, "s3cret")
		if err := cp.ConfirmEnvironmentTeardown(context.Background(), "cell-0", "env_x"); err == nil {
			t.Errorf("a %d was reported as a successful confirmation — the control plane "+
				"refused and the agent would stop retrying", code)
		}
		ts.Close()
	}
}

// An unmounted route 404s, which must surface — this is what catches a client
// that posts to a path the server does not serve.
func TestConfirmEnvironmentTeardownFailsOnAnUnservedPath(t *testing.T) {
	ts := httptest.NewServer(http.NewServeMux()) // nothing mounted
	defer ts.Close()
	cp := NewHTTPControlPlane(ts.URL, "s3cret")
	if err := cp.ConfirmEnvironmentTeardown(context.Background(), "cell-0", "env_x"); err == nil {
		t.Fatal("posting to a path nothing serves was reported as success")
	}
}
