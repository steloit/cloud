package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Chain's ORDER is a correctness property, not a style choice, so it is pinned
// rather than left to the reading of main.go.

func mark(name string, order *[]string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*order = append(*order, name)
			next.ServeHTTP(w, r)
		})
	}
}

func TestIdempotencyRunsBeforeSSEAndTheMux(t *testing.T) {
	var order []string
	mux := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		order = append(order, "mux")
	})
	h := Chain(mux, mark("idem", &order), mark("sse", &order))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/estimates", nil))

	got := strings.Join(order, ">")
	// idem must see the request first: it needs the raw body to fingerprint and
	// the raw response to replay, neither of which survives below SSE.
	if got != "idem>sse>mux" {
		t.Fatalf("chain order is %q, want %q", got, "idem>sse>mux")
	}
}

// A panic anywhere below must still reach the client as problem+json — so
// problem.Recover has to be the OUTERMOST layer, above idempotency.
func TestRecoverIsOutermost(t *testing.T) {
	mux := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	h := Chain(mux, mark("idem", &[]string{}), nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/estimates", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("a panic must become a 500, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "remediation") {
		t.Fatalf("a panic must still produce problem+json with remediation: %s", w.Body.String())
	}
}

// A harness that does not exercise a layer omits it explicitly; a nil must be a
// no-op, never a nil-deref that only shows up in that harness.
func TestNilMiddlewaresAreNoOps(t *testing.T) {
	mux := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	})
	w := httptest.NewRecorder()
	Chain(mux, nil, nil).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != 204 {
		t.Fatalf("nil middlewares must pass through, got %d", w.Code)
	}
}
