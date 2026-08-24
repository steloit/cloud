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
	for _, c := range everyPair() {
		o := ObservedStatus(c.from, c.observed)
		to, ok := o.Edge()
		if !ok {
			// NOT `continue`. "No change" is legal from everywhere, so skipping
			// here is what made this sweep structurally blind to the case where
			// no change is the WRONG answer — the harm assertions below run on
			// this branch too, and that is where the real defects lived.
			if to != c.from {
				t.Errorf("from %q observing %q reports no edge but names %q as the destination — "+
					"a no-op must leave the status alone", c.from, c.observed, to)
			}
			continue
		}
		if !CanTransition(c.from, to) {
			t.Errorf("from %q observing %q yields the edge %q, which ADR-024 does not allow "+
				"from %q — Transition would 409 every tick and the row would be retried "+
				"forever with nothing visible", c.from, c.observed, to, c.from)
		}
		if !statusCheckVocab[to] {
			t.Errorf("from %q observing %q yields %q, which the services CHECK constraint "+
				"refuses — bad input would become a 500", c.from, c.observed, to)
		}
	}
}

// The ADR-024 vocabulary, from the ONE definition rather than a fourth retyping.
// The integration suite pins StatusVocabulary against the real CHECK constraint.
var statusCheckVocab = func() map[string]bool {
	v := map[string]bool{}
	for _, st := range StatusVocabulary() {
		v[st] = true
	}
	return v
}()

type pair struct{ from, observed string }

// everyPair is the TOTAL input domain, and the count is asserted rather than
// floored. The previous sweep guarded itself with `if checked < 30`, which
// passed at 36 while an entire column of the vocabulary (`suspended`) was
// missing from the report list — a self-check that cannot see what was never
// enumerated is not a self-check.
func sweepDomain() (froms, observeds []string) {
	froms = []string{"provisioning", "ready", "degraded", "failed", "suspended", "deleting", "", "not-a-status"}
	// Everything reconcile/http.go's statusVocab admits on the wire, plus junk.
	// `deleting` and `suspended` are in it because that vocabulary mirrors the
	// customer-facing enum — that they are ACCEPTED on the wire is exactly why
	// the mapping has to answer for them.
	observeds = []string{"provisioning", "ready", "degraded", "failed", "suspended", "deleting", "gone", "", "not-a-status"}
	return froms, observeds
}

func everyPair() []pair {
	froms, observeds := sweepDomain()
	out := make([]pair, 0, len(froms)*len(observeds))
	for _, f := range froms {
		for _, o := range observeds {
			out = append(out, pair{f, o})
		}
	}
	return out
}

// The sweep must cover the WHOLE vocabulary, not merely a lot of pairs.
//
// A count is not coverage: `len(everyPair()) == 8*8` stayed green with the
// entire `suspended` row, the entire `suspended` column and the empty-`from` row
// replaced by junk strings, because the product still came to 64. That is the
// same defect as the `if checked < 30` floor this sweep replaced — a self-check
// cannot see what was never enumerated. So assert MEMBERSHIP.
func TestThePairSweepCoversTheWholeVocabulary(t *testing.T) {
	has := func(hay []string, needle string) bool {
		for _, h := range hay {
			if h == needle {
				return true
			}
		}
		return false
	}
	froms, observeds := sweepDomain()
	for status := range statusCheckVocab {
		if !has(froms, status) {
			t.Errorf("the services CHECK constraint allows %q and no pair starts from it", status)
		}
		if !has(observeds, status) {
			t.Errorf("reconcile's statusVocab admits %q on the wire and no pair observes it", status)
		}
	}
	for _, special := range []string{"", "not-a-status"} {
		if !has(froms, special) {
			t.Errorf("no pair starts from %q", special)
		}
		if !has(observeds, special) {
			t.Errorf("no pair observes %q", special)
		}
	}
	if !has(observeds, "gone") {
		t.Error("no pair observes `gone`, which is the teardown signal on the wire")
	}
	if got, want := len(everyPair()), len(froms)*len(observeds); got != want {
		t.Fatalf("the sweep covers %d pairs, want the full product %d", got, want)
	}
	seen := map[pair]bool{}
	for _, c := range everyPair() {
		if seen[c] {
			t.Fatalf("duplicate pair %+v — a duplicate inflates the product without covering anything", c)
		}
		seen[c] = true
	}
}

// THE HALF THE SWEEPS ABOVE CANNOT SEE: SOMETHING MUST ACTUALLY CONVERGE.
//
// Every other invariant here is guarded by `if !o.Converged() { continue }`, so
// they can only ever punish converging too EAGERLY. Measured: replacing the
// whole body of ObservedStatus with `return Observation{}` — a mapping that does
// nothing for all 72 pairs — left all of them green. Under-convergence is the
// ORIGINAL defect of this task (409 every tick, retried forever, nothing
// visible), so without this the sweep is blind to half the harm it claims.
//
// Stated as the harm, not as the implementation: if the cell reported a settled
// status and the row now rests on exactly that status, there is nothing left to
// watch and the generation MUST be finished. Anything else means the agent
// re-applies that service forever.
//
// Held rows are excluded because they settle on different evidence (step 1: only
// a confirmed teardown finishes a hold), which the held-row tests pin directly.
func TestAnObservationThatComesToRestOnWhatWasReportedFinishesTheGeneration(t *testing.T) {
	converging := 0
	for _, c := range everyPair() {
		if c.from == "deleting" || c.from == "suspended" {
			continue
		}
		o := ObservedStatus(c.from, c.observed)
		to, ok := o.Edge()
		final := c.from
		if ok {
			final = to
		}
		if final != c.observed || !settledStatuses[final] {
			continue
		}
		converging++
		if !o.Converged() {
			t.Errorf("from %q observing %q comes to rest on %q — a settled status the cell "+
				"itself reported — and does NOT finish the generation. The row stays in "+
				"ListDesiredForCell and the agent re-applies it every tick forever.",
				c.from, c.observed, final)
		}
	}
	if converging == 0 {
		t.Fatal("no pair in the whole domain comes to rest on a settled reported status — the " +
			"mapping is inert and every other invariant here would still pass")
	}
}

// A CONVERGED ROW LEAVES THE OUTSTANDING SET. It must therefore never come to
// rest anywhere that still needs watching.
//
// This is the harm stated independently of how the mapping computes it, and it
// is the single property that kills all four defects review found: `ready` +
// `degraded` settling on a billing state in one hop, `ready` + `failed` doing
// the same before round 2 fixed that hop ALONE, a transient `provisioning`
// finishing a generation mid-apply, and an unplaceable report finishing one at
// a stale status.
//
// `degraded` BILLS (metering.IsBilling) and `degraded → failed` is the only edge
// that emits a metering `close`, so a row resting at `degraded` that nothing
// observes again bills indefinitely.
func TestAConvergedObservationNeverRestsOnAStateStillBeingWatched(t *testing.T) {
	for _, c := range everyPair() {
		o := ObservedStatus(c.from, c.observed)
		if !o.Converged() {
			continue
		}
		to, ok := o.Edge()
		final := c.from
		if ok {
			final = to
		}
		if final == "provisioning" || final == "degraded" {
			t.Errorf("from %q observing %q CONVERGES on %q — the row leaves the outstanding set "+
				"while still %s, and nothing ever observes it again",
				c.from, c.observed, final,
				map[string]string{"provisioning": "mid-apply", "degraded": "impaired AND BILLING"}[final])
		}
	}
}

// AND IT NEVER RESTS ON A STATUS THE CELL DID NOT REPORT.
//
// The other half of the same harm: advancing observed_generation onto a state
// nobody observed drops the row out of ListDesiredForCell for good. `ready` +
// `provisioning` (mid-apply, no legal edge) and `provisioning` + `degraded` (no
// edge at all) both used to settle at `from` and strand the service silently.
//
// The two lifecycle holds are the deliberate exception, and they are exempted BY
// NAME rather than by a blanket skip: for those the report genuinely cannot
// matter, and keeping the row outstanding would 409 forever with no edge that
// could end it.
func TestAConvergedObservationRestsOnWhatTheCellReported(t *testing.T) {
	for _, c := range everyPair() {
		if c.from == "deleting" || c.from == "suspended" || c.observed == "" || c.observed == "gone" {
			continue // the held/terminal and non-status observations, asserted on their own below
		}
		o := ObservedStatus(c.from, c.observed)
		if !o.Converged() {
			continue
		}
		to, ok := o.Edge()
		final := c.from
		if ok {
			final = to
		}
		if final != c.observed {
			t.Errorf("from %q observing %q CONVERGES resting on %q, which the cell never reported — "+
				"observed_generation advances onto a status nothing observed and the row is "+
				"never reconciled again", c.from, c.observed, final)
		}
	}
}

// A CELL CANNOT SUSPEND OR DELETE A SERVICE.
//
// `deleting` and `suspended` are lifecycle decisions the control plane makes;
// the agent's statusFromPhase cannot produce either. They reach the mapping only
// because reconcile/http.go's statusVocab mirrors the customer-facing enum.
//
// Left ungated, CanTransition accepts BOTH straight from `ready`, and one POST
// with the reconciler token bricks a service permanently: the edge lands, but
// SetServiceStatus does not bump the generation, so no `deleting:true` desired
// doc is produced and no teardown runs; `deleting` has no outgoing edge; and
// DeleteService then answers "deletion already in progress" forever — metering
// span closed, workload still running.
func TestAnObservationCanNeverSuspendOrDeleteAService(t *testing.T) {
	for _, from := range []string{"provisioning", "ready", "degraded", "failed"} {
		for _, observed := range []string{"deleting", "suspended"} {
			o := ObservedStatus(from, observed)
			if to, ok := o.Edge(); ok {
				t.Errorf("a cell observing %q moved a %q service to %q — suspension and deletion "+
					"are control-plane intents, never observations", observed, from, to)
			}
			// And it must NOT settle either: settling is the quieter attack —
			// observed_generation advances and the row silently stops being
			// reconciled, for every service the token's cell list covers.
			if o.Converged() {
				t.Errorf("a cell observing %q from %q CONVERGED the generation — a refused report "+
					"must leave the row outstanding", observed, from)
			}
		}
	}
}

// The refusal has TWO representations and they must not drift: the mapping above
// and the HTTP boundary the reconciler token actually reaches
// (reconcile.Handlers.status returns 422). Both read this predicate.
func TestReportableByCellExcludesExactlyTheLifecycleStates(t *testing.T) {
	for _, s := range []string{"provisioning", "ready", "degraded", "failed"} {
		if !ReportableByCell(s) {
			t.Errorf("%q is a workload observation a cell must be able to report", s)
		}
	}
	for _, s := range []string{"deleting", "suspended", "", "gone", "not-a-status"} {
		if ReportableByCell(s) {
			t.Errorf("%q is not something a cell can observe about a workload", s)
		}
	}
}

// EVERY OBSERVATION REACHES A FIXED POINT, so no mapping can cycle.
//
// Feed the destination back as `from` with the SAME observation, the way
// successive ticks of an unchanging cluster do. `to` must stop moving within two
// hops — the two-hop paths are `ready`+`failed` → degraded → failed and
// `failed`+`ready` → provisioning → ready.
//
// REACHING A FIXED POINT IS NOT THE SAME AS CONVERGING, and this test does not
// claim it is. `degraded` + `degraded` and `provisioning` + `provisioning` take
// no edge and never converge: the row stays in ListDesiredForCell and the agent
// re-applies it every tick, forever. That is deliberate (an impaired row must
// stay watched — `degraded` bills) and it is filed as US-3.13, which also
// records that US-3.11 does NOT cover it.
//
// What this rules out is narrower and worth ruling out on its own: the mapping
// oscillating between statuses while the cluster reports the same thing every
// tick. The positive sweep above owns "something must actually converge".
func TestEveryObservationReachesAFixedPointWithinTwoHops(t *testing.T) {
	for _, c := range everyPair() {
		from := c.from
		var path []string
		for hop := 0; hop < 4; hop++ {
			o := ObservedStatus(from, c.observed)
			to, ok := o.Edge()
			if !ok {
				break
			}
			path = append(path, to)
			if to == from {
				t.Fatalf("from %q observing %q reports an edge to itself", c.from, c.observed)
			}
			from = to
		}
		if len(path) > 2 {
			t.Errorf("from %q observing %q walks %v — more than two hops means the mapping "+
				"can cycle while the cluster reports the same thing every tick",
				c.from, c.observed, path)
		}
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
	// NOT converged — and the reason is billing, not tidiness. An earlier version
	// of this test asserted the opposite with the justification "marking it
	// unconverged would keep the row outstanding forever", which is measurably
	// false: ObservedStatus("degraded","failed") IS converged, so it terminates
	// in two hops. What converging here would actually do is advance
	// observed_generation, drop the row out of the outstanding set, and leave an
	// unrecoverable cluster resting at `degraded` — which BILLS — with nothing
	// ever observing it again. `degraded → failed` is the only edge that closes
	// a span.
	if o.Converged() {
		t.Error("ready → degraded was marked converged: the row leaves the outstanding set and " +
			"an unrecoverable cluster bills forever at `degraded`, because degraded → failed is " +
			"the only edge that closes the metering span")
	}
	// The second hop terminates it, either way.
	recovered := ObservedStatus("degraded", "ready")
	if to, ok := recovered.Edge(); !ok || to != "ready" || !recovered.Converged() {
		t.Errorf("degraded + ready = (%q, %v, converged=%v), want (ready, true, true)",
			to, ok, recovered.Converged())
	}
	stillBroken := ObservedStatus("degraded", "failed")
	if to, ok := stillBroken.Edge(); !ok || to != "failed" || !stillBroken.Converged() {
		t.Errorf("degraded + failed = (%q, %v, converged=%v), want (failed, true, true) — this is "+
			"the hop that closes the billing span", to, ok, stillBroken.Converged())
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

// `gone` IS NEVER A STATUS EDGE — BUT IT ONLY FINISHES A GENERATION WHEN THE
// TEARDOWN IS THE ONE WE ASKED FOR.
//
// Row removal stays the deletion pipeline's job (US-3.5) in every case, so there
// is no edge from any `from`. Convergence is the half the old test got wrong: it
// asserted converged for `ready` and `provisioning` too, under the name
// "completed teardown", when for those states `gone` means the workload VANISHED
// while desired still wants it alive. Settling there advances
// observed_generation on a service that no longer exists — it leaves
// ListDesiredForCell for good, the customer keeps seeing `ready`, and metering
// keeps billing a `ready` span with nothing running.
func TestGoneFinishesAGenerationOnlyWhenTheTeardownWasIntended(t *testing.T) {
	for _, from := range []string{"deleting", "suspended", "ready", "provisioning", "degraded", "failed"} {
		o := ObservedStatus(from, "gone")
		if to, ok := o.Edge(); ok {
			t.Errorf("from %q, `gone` produced the edge %q — row removal is the deletion "+
				"pipeline's job, never a status edge", from, to)
		}
		intended := from == "deleting" || from == "suspended"
		if got := o.Converged(); got != intended {
			if intended {
				t.Errorf("from %q, `gone` was marked unconverged — the row would stay outstanding "+
					"forever after the teardown we asked for", from)
			} else {
				t.Errorf("from %q, `gone` CONVERGED — the workload vanished while desired still "+
					"wants it alive, and the row just left the outstanding set, so nothing will "+
					"ever re-create it", from)
			}
		}
	}
}

// AN EMPTY STATUS IS NOT `gone`. The handler used to normalise the wire's `gone`
// into "", so the machine could not tell "the thing you asked me to run does not
// exist" from "I converged this generation and have nothing to say about its
// status".
//
// They diverge on a SETTLED row, which is where the collapse did its damage: a
// `ready` service whose workload has vanished must stay outstanding so the agent
// re-creates it, while a `ready` service whose agent simply had nothing to
// report is finished. Under the collapse both were finished.
//
// On an UNSETTLED row (`provisioning`, `degraded`) they agree that the
// generation is not done — but for different reasons, and step 4 reaches that
// answer for the ack while step 1b reaches it for `gone`.
func TestAnObservationOnlyAckIsNotTheSameAsGone(t *testing.T) {
	for _, from := range []string{"ready", "failed"} {
		ack, gone := ObservedStatus(from, ""), ObservedStatus(from, "gone")
		if !ack.Converged() {
			t.Errorf("from %q, the observation-only ack did not converge — the agent applied the "+
				"generation, the row rests on a settled status, and there is nothing left to watch", from)
		}
		if gone.Converged() {
			t.Errorf("from %q, `gone` CONVERGED — the workload vanished while desired still wants "+
				"it alive, and the row just left the outstanding set", from)
		}
	}
	// And neither is ever an edge, from anywhere.
	for _, from := range []string{"provisioning", "ready", "degraded", "failed", "suspended", "deleting"} {
		for _, observed := range []string{"", "gone"} {
			if to, ok := ObservedStatus(from, observed).Edge(); ok {
				t.Errorf("from %q, observing %q produced the edge %q — neither is a status", from, observed, to)
			}
		}
	}
}

// AN ACK CANNOT PARK AN UNSETTLED ROW. "I applied the generation, no status to
// report" asserts nothing about where the row rests, so it must not finish a
// generation on a status that is still being watched — `degraded` BILLS, and a
// row parked there unwatched bills indefinitely.
func TestAnAckDoesNotFinishAGenerationOnAnUnsettledRow(t *testing.T) {
	for _, from := range []string{"provisioning", "degraded"} {
		if ObservedStatus(from, "").Converged() {
			t.Errorf("from %q, an empty observation CONVERGED — the row leaves the outstanding set "+
				"while still %q, and nothing ever observes it again", from, from)
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
