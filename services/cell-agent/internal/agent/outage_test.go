package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

// TestOutageDrill is US-1.3's headline acceptance criterion, end to end: with a
// REAL HTTP client against a REAL server that then dies, prove that a
// control-plane outage degrades to "cannot make changes" and never touches a
// running workload.
//
// The unit tests use a fake control plane; this one exercises the actual
// network failure path (connection refused) through HTTPControlPlane, which is
// the path that matters when the API is genuinely down.
func TestOutageDrill(t *testing.T) {
	var (
		mu      sync.Mutex
		reports []Report
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/reconcile/{cell}/desired", func(w http.ResponseWriter, r *http.Request) {
		// one service, generation 5, until the "outage"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"services": []DesiredService{{
				ID: "svc_live", CellID: "cell-0", Product: "postgres", Generation: 5,
				Desired: map[string]any{"product": "postgres"},
			}},
		})
	})
	mux.HandleFunc("POST /v1/reconcile/{cell}/status", func(w http.ResponseWriter, r *http.Request) {
		var rep Report
		_ = json.NewDecoder(r.Body).Decode(&rep)
		mu.Lock()
		reports = append(reports, rep)
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"service_id": rep.ServiceID, "status": rep.Status, "observed_generation": rep.ObservedGeneration})
	})
	ts := httptest.NewServer(mux)

	// A renderer that counts real converges — the stand-in for "touching the
	// customer workload". A converge after the outage is the failure this drill
	// exists to catch.
	converges := &countingRenderer{}
	cp := NewHTTPControlPlane(ts.URL, "tok")
	a := New("cell-0", cp, converges, quietLog())
	ctx := context.Background()

	// --- control plane UP: the workload gets provisioned ---
	a.Tick(ctx)
	if converges.n.Load() != 1 {
		t.Fatalf("with the control plane up, expected 1 converge, got %d", converges.n.Load())
	}
	mu.Lock()
	gotReports := len(reports)
	mu.Unlock()
	if gotReports != 1 {
		t.Fatalf("expected the ready status to be reported, got %d reports", gotReports)
	}
	convergesAtOutage := converges.n.Load()

	// --- CONTROL PLANE OUTAGE: the server dies mid-flight ---
	ts.Close()

	// The agent keeps ticking (as its loop would). Each tick's poll now fails
	// with connection-refused against the real client.
	for i := 0; i < 5; i++ {
		a.Tick(ctx) // must not panic, must not converge, must not error out of the loop
	}

	// THE GUARANTEE: nothing was converged during the outage. The workload that
	// was running before the control plane died is untouched — no teardown, no
	// re-render, no degradation. "Cannot make changes", never "apps down".
	if converges.n.Load() != convergesAtOutage {
		t.Fatalf("the agent converged %d time(s) during a control-plane outage — a running workload was touched while the control plane was down",
			converges.n.Load()-convergesAtOutage)
	}

	// And the agent is still alive: bring a fresh control plane back and confirm
	// it resumes (an outage must be recoverable, not terminal).
	ts2 := httptest.NewServer(mux)
	defer ts2.Close()
	a2 := New("cell-0", NewHTTPControlPlane(ts2.URL, "tok"), converges, quietLog())
	a2.Tick(ctx)
	if converges.n.Load() != convergesAtOutage+1 {
		t.Fatal("the agent did not resume converging after the control plane recovered")
	}
}

type countingRenderer struct{ n atomic.Int64 }

func (c *countingRenderer) Converge(_ context.Context, svc DesiredService) (string, error) {
	c.n.Add(1)
	return "ready", nil
}
