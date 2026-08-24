package provisioning

// US-3.3h: the cell reports what it OBSERVES; this maps it onto a status legal
// from what the service currently IS.

import "testing"

// EVERY (from × observed) PAIR LANDS SOMEWHERE THE MACHINE ACCEPTS.
//
// This is the property the defect violated: `statusFromPhase` answers from the
// phase alone, so a cluster that broke while READY reported `failed`, which
// ADR-024 does not allow from `ready` — Transition rejected it every tick,
// observed_generation never advanced, and the service was retried forever with
// nothing visible.
func TestEveryObservationLandsOnALegalEdgeOrNoChange(t *testing.T) {
	states := []string{"provisioning", "ready", "degraded", "failed", "suspended", "deleting"}
	// What a cell can actually report: the terminal statuses the driver emits,
	// plus "" (the wire's `gone`, normalised upstream).
	reports := []string{"ready", "failed", "degraded", "provisioning", "deleting", ""}

	checked := 0
	for _, from := range states {
		for _, observed := range reports {
			o := ObservedStatus(from, observed)
			to, ok := o.Edge()
			checked++
			if !ok {
				continue // no change is always safe
			}
			if !CanTransition(from, to) {
				t.Errorf("from %q observing %q yields the edge %q, which ADR-024 does not allow "+
					"from %q — Transition would 409 every tick and the row would be retried "+
					"forever with nothing visible", from, observed, to, from)
			}
		}
	}
	if checked < 30 {
		t.Fatalf("only %d pairs checked — this test would prove little", checked)
	}
}

// LEGAL IS NOT THE SAME AS RIGHT. A legality sweep skips any answer equal to
// `from` (no change is legal from everywhere), so it is structurally blind to
// the case where no change is the WRONG answer: a cluster that breaks while
// READY reported `ready` forever is worse than the 409 loop — no writeback, no
// alert, a broken database indistinguishable from a healthy one.
func TestABrokenReadyClusterBecomesDegradedNotUnchanged(t *testing.T) {
	o := ObservedStatus("ready", "failed")
	to, ok := o.Edge()
	if !ok || to != "degraded" {
		t.Fatalf("ready + failed = (%q, %v), want (degraded, true)", to, ok)
	}
	if !o.Converged() {
		t.Error("ready → degraded is a single legal hop; marking it unconverged would keep the " +
			"row outstanding forever")
	}
	// ...and the converse, so this cannot be satisfied by always answering degraded.
	if to, ok := ObservedStatus("ready", "ready").Edge(); ok {
		t.Errorf("a healthy cluster under a ready row produced the edge %q", to)
	}
}

// THE ONE HOP THAT IS NOT CONVERGED. ADR-024 has `failed → {provisioning,
// deleting}` — there is no failed → ready — so a healthy cluster under a failed
// row routes through `provisioning`. Marking it converged strands the row there
// forever, because ListDesiredForCell selects on observed_generation < generation.
func TestARecoveredFailedServiceRoutesThroughProvisioningAndIsNotConverged(t *testing.T) {
	o := ObservedStatus("failed", "ready")
	to, ok := o.Edge()
	if !ok || to != "provisioning" {
		t.Fatalf("failed + ready = (%q, %v), want (provisioning, true)", to, ok)
	}
	if o.Converged() {
		t.Fatal("the failed→ready hop was marked CONVERGED. observed_generation would advance, " +
			"the row would leave the outstanding set at `provisioning`, and it would never reach " +
			"ready — stranded in exactly the state the retry path was meant to pass through")
	}
	// The second tick finishes it.
	next := ObservedStatus("provisioning", "ready")
	to2, ok2 := next.Edge()
	if !ok2 || to2 != "ready" || !next.Converged() {
		t.Fatalf("the next tick gave (%q, %v, converged=%v), want (ready, true, true)",
			to2, ok2, next.Converged())
	}
}

// A SUSPENDED SERVICE IS NEVER AUTO-RESUMED. `suspended → ready` IS a legal
// edge, so without an explicit arm a converging agent that sees a healthy
// cluster silently un-suspends the service and restarts its metering span.
// Something suspended it; observing health is not consent to resume.
func TestASuspendedServiceIsNeverResumedByAnObservation(t *testing.T) {
	for _, observed := range []string{"ready", "degraded", "failed", "provisioning"} {
		if to, ok := ObservedStatus("suspended", observed).Edge(); ok {
			t.Errorf("a suspended service observing %q was moved to %q — nothing asked for that, "+
				"and if it lands on `ready` the metering span restarts", observed, to)
		}
	}
}

// A `from` OUTSIDE THE MACHINE IS NEVER ECHOED BACK. reconcile/http.go's
// statusVocab would reject it and the services CHECK constraint would refuse the
// UPDATE, so returning it as an edge turns a bad input into a 500.
func TestAnUnknownFromStateNeverProducesAnEdge(t *testing.T) {
	for _, from := range []string{"", "gone", "deleted", "READY", " ready", "ready\n", "🙂"} {
		for _, observed := range []string{"ready", "failed", "provisioning"} {
			if to, ok := ObservedStatus(from, observed).Edge(); ok {
				t.Errorf("from %q observing %q produced the edge %q — not a status the machine, "+
					"the writeback vocabulary or the DB CHECK will accept", from, observed, to)
			}
		}
	}
}

// `gone` IS OBSERVATION-ONLY. reconcile/http.go normalises the wire's `gone` to
// "", because a completed teardown is not a status edge — the row's terminal
// state is `deleting` and removal is the deletion pipeline's job. Handled in the
// mapping rather than left to a caller's guard, because a caller's guard is what
// this type exists to make unnecessary.
func TestACompletedTeardownIsNotAStatusEdge(t *testing.T) {
	for _, from := range []string{"deleting", "ready", "provisioning"} {
		o := ObservedStatus(from, "")
		if to, ok := o.Edge(); ok {
			t.Errorf("from %q, an empty observation produced the edge %q", from, to)
		}
		if !o.Converged() {
			t.Errorf("from %q, an empty observation was marked unconverged — the row would stay "+
				"outstanding forever after a completed teardown", from)
		}
	}
}

// The zero value must be inert: a caller that forgets to call ObservedStatus
// does nothing, rather than something wrong.
func TestTheZeroObservationDoesNothing(t *testing.T) {
	var o Observation
	if to, ok := o.Edge(); ok {
		t.Errorf("the zero Observation reports the edge %q", to)
	}
	if o.Converged() {
		t.Error("the zero Observation reports converged — a caller that never consulted the " +
			"machine would advance observed_generation")
	}
}
