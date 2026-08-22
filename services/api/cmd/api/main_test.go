package main

import (
	"context"
	"testing"
)

// The composition root's ONLY test, and it exists for one reason: before it,
// `cmd/api` had no test files, so deleting a `go worker.Run(ctx, …)` line from
// main left `go test ./...` fully green. Each worker's BEHAVIOUR is covered
// elsewhere, but every one of those tests starts the goroutine itself — nothing
// asserted production ever does.
//
// This asserts the registry by NAME. A worker deleted from backgroundWorkers is
// a failure here; a worker added without a name is a compile error at the map
// literal. Neither is a silent no-op, which is the property being bought.
func TestEveryBackgroundWorkerIsRegistered(t *testing.T) {
	// Spelled out, not derived from the map under test — deriving the
	// expectation from the thing being tested asserts a function against
	// itself, and this repo has a mistake-bank entry for exactly that.
	want := []string{
		"mailer.outbox",
		"subscription.lifecycle",
		"notify.outbox",
		"notify.bell",
		"idempotency.sweeper",
		"provisioning.override-expiry",
	}

	// nil components are fine: the registry closes over them and this test
	// never calls the closures. Constructing real ones would need a database,
	// and would test the constructors rather than the wiring.
	got := backgroundWorkers(nil, nil, nil, nil, nil, nil)

	for _, name := range want {
		run, ok := got[name]
		if !ok {
			t.Errorf("background worker %q is not registered — it does not run in production, "+
				"and every test of its behaviour starts the goroutine itself, so nothing else will catch this", name)
			continue
		}
		if run == nil {
			t.Errorf("background worker %q is registered but nil — main would start a goroutine that does nothing", name)
		}
	}

	// Both directions. A worker added to the registry without being added here
	// is not a defect, but it IS an unreviewed long-lived goroutine in the
	// process, and this is the one place that fact is visible.
	if len(got) != len(want) {
		t.Errorf("the registry holds %d workers but this test knows %d — a long-lived goroutine was added "+
			"or removed without updating the list that makes it visible: %v", len(got), len(want), keysOf(got))
	}
}

func keysOf(m map[string]func(context.Context)) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
