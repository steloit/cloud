// Package httpapi owns the server's outermost handler assembly.
//
// It exists so the composition root and the integration harness cannot build
// DIFFERENT servers. Before this, each duplicated the chain by hand, which
// meant a middleware could be mounted in production and absent from every test
// (or the reverse) with nothing failing — the tests would prove a server that
// is not the one that ships.
package httpapi

import (
	"net/http"

	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// Middleware wraps a handler. A nil Middleware is a no-op, so a harness that
// does not exercise a layer can omit it explicitly rather than by forgetting.
type Middleware func(http.Handler) http.Handler

// Chain assembles the server. ORDER IS LOAD-BEARING:
//
//   - problem.Recover is OUTERMOST: a panic anywhere below must still leave the
//     client a problem+json response, never a bare connection reset.
//   - idempotency is above SSE and the mux: it must see the raw request body
//     (to fingerprint it) and the raw response bytes (to replay them verbatim),
//     which nothing below the mux can offer.
//   - sse is innermost: it hijacks the ResponseWriter for streams, so anything
//     wrapping a stream response must already have decided not to buffer it.
func Chain(mux http.Handler, idem, sse Middleware) http.Handler {
	h := mux
	if sse != nil {
		h = sse(h)
	}
	if idem != nil {
		h = idem(h)
	}
	return problem.Recover(h)
}
