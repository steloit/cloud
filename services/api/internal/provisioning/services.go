package provisioning

// T3.3: service rows + the guarded status machine (ADR-024 vocabulary:
// provisioning|ready|degraded|failed|suspended|deleting — `ready`, never
// `running`; metering starts at ready). Rows are DESIRED STATE (D9): the
// reconciler (T3.4+, cell-agent) converges actual state; nothing here talks
// to infrastructure.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/steloit/cloud/services/api/internal/billing"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/steloit/cloud/services/api/internal/estimates"
	"github.com/steloit/cloud/services/api/internal/events"
	"github.com/steloit/cloud/services/api/internal/identity/store"
	"github.com/steloit/cloud/services/api/internal/metering"
	"github.com/steloit/cloud/services/api/internal/platform/ids"
	"github.com/steloit/cloud/services/api/internal/platform/money"
	"github.com/steloit/cloud/services/api/internal/platform/problem"
)

// enforceBudget is the hard spend cap (T11.6, F9): before an estimate is
// accepted, project the org's committed monthly run-rate (plan fee + Σ active
// services' monthly estimates) plus this service's monthly cost against the
// bound. Over the bound → refuse with the arithmetic shown. Uncapped (no row or
// a null limit) → always proceeds. Running services are NEVER touched — the cap
// pauses new provisioning only (US-11.5: cancel/pause ≠ delete).
func (s *Service) enforceBudget(ctx context.Context, orgID string, newMonthly money.Cents) error {
	budget, err := s.q.GetBudget(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) || !budget.LimitCents.Valid {
		return nil // uncapped
	}
	if err != nil {
		return err
	}
	limit := budget.LimitCents.Int64
	org, err := s.q.GetOrg(ctx, orgID)
	if err != nil {
		return err
	}
	var planFee int64
	if fee, ok := s.plans.PlanFeeCents(org.Plan); ok {
		planFee = int64(fee)
	}
	committed, err := s.q.SumOrgMonthlyEstimate(ctx, orgID)
	if err != nil {
		return err
	}
	// The PROJECTION is money arithmetic, so it cannot wrap — and the SUMMANDS
	// are re-validated on the way in, which is the part that matters most.
	// `committed` comes from SumOrgMonthlyEstimate over stored rows; if any row
	// holds a value outside the representable range (which a past wrap could
	// leave behind, and did), FromInt refuses and the cap fails CLOSED. Before
	// this, one wrapped row disabled an org's cap permanently, because every
	// later projection wrapped too.
	feeAmt, feeErr := money.FromInt(planFee)
	committedAmt, cErr := money.FromInt(committed)
	// `limit` goes through FromInt too. It is a STORED value like the others, and
	// leaving it as a raw int64 on the right-hand side of the comparison meant an
	// out-of-range limit_cents was evaluated silently while an out-of-range
	// committed failed closed — the same input class treated two ways.
	limitAmt, lErr := money.FromInt(limit)
	if feeErr != nil || cErr != nil || lErr != nil {
		return problemError{p: problem.Conflict(
			[]string{"this organization's committed monthly spend is not a valid amount"},
			"Contact support: a stored monthly estimate or budget is outside the representable range, so the spend cap cannot be evaluated safely.")}
	}
	current, curErr := feeAmt.Add(committedAmt)
	projected, projErr := current.Add(newMonthly)
	// Overflow is treated as OVER CAP — the only safe direction: a number we
	// cannot represent is not a number we can prove is affordable.
	notRepresentable := curErr != nil || projErr != nil
	if notRepresentable || projected.GreaterThan(limitAmt) {
		// F9 flagship: an ENFORCED bound, refused at accept time (402) with the
		// arithmetic shown — never an alert-only. Every cap hit lands on the
		// events spine (AC3) so "the cap is real" is auditable, not just a UI toast.
		// On the not-representable branch `current` and `projected` are Zero,
		// because that is what a failed checked add returns. Printing them would
		// put "current $0.00, projected $0.00" on the audit spine for precisely
		// the anomalous case the spine exists to record — AC3 asks that the cap
		// being real be AUDITABLE, and a row of zeros is worse than no row.
		detail := fmt.Sprintf(`{"cap_cents":%d,"current_cents":%d,"requested_cents":%d,"projected_cents":%d}`,
			limitAmt.Int64(), current.Int64(), newMonthly.Int64(), projected.Int64())
		if notRepresentable {
			// current_cents and projected_cents are OMITTED, not null. Both mean
			// "not computable", and using two encodings for that in one object is
			// how a consumer gets it wrong: a reader shaped like
			// budget_integration_test.go's (`ProjectedCents int64`) decodes null
			// to 0 — indistinguishable from a real zero, which is precisely the
			// "row of zeros" this branch exists to avoid. Absence uniformly means
			// not computable, and `reason` carries the why.
			detail = fmt.Sprintf(`{"cap_cents":%d,"requested_cents":%d,"reason":"not_representable"}`,
				limitAmt.Int64(), newMonthly.Int64())
		}
		s.record(ctx, events.Input{
			OrgID: orgID, Kind: "billing", Via: "system", Actor: "system",
			Action: "billing.spend_cap_reached", Subject: orgID,
			Detail: []byte(detail),
		})
		// The hard spend cap is a 402 quota_exceeded (the x-error-catalog's
		// sanctioned "hard quota: fails with remediation" — NOT a new error
		// class): an ENFORCED bound refused at accept time with the arithmetic,
		// never an alert-only.
		msg := fmt.Sprintf("this raises your monthly spend to %s (current %s + this service %s), above your %s cap",
			projected, current, newMonthly, limitAmt)
		remediation := "Raise the budget in Billing, or provision a smaller shape — nothing running is affected."
		if notRepresentable {
			// Do NOT claim the projection exceeds the cap: this branch is reached
			// precisely because the projection could not be computed. And do not
			// offer "raise the budget" — raising it cannot resolve an arithmetic
			// overflow, and api-conventions requires each failure to name a next
			// step that can actually work.
			msg = fmt.Sprintf("this service is priced at %s, and your organization's committed monthly spend cannot be evaluated — the total is outside the range the platform can represent exactly",
				newMonthly)
			// NOT "a stored estimate is out of range" — this branch is reached
			// only after feeErr/cErr/lErr all passed, so every stored value is in
			// range by construction and it is their SUM that overflows. Had a
			// stored value been out of range, the 409 above would have answered.
			remediation = "Provision a smaller shape, or contact support: the total of this organization's committed monthly estimates is larger than the platform can represent exactly."
		}
		return problemError{p: problem.QuotaHard(msg, remediation)}
	}
	return nil
}

// StatusVocabulary is the ADR-024 status set, and it is the ONE definition.
//
// It was retyped in four places before US-3.3h round 4 — reconcile's wire gate
// and three separate test sweeps — with nothing tying them, so a status added
// here would have been swept by none of them. reconcile builds its wire vocab
// from this, and the sweeps assert membership against it.
//
// It mirrors the services CHECK constraint
// (platform/db/migrations/20260718203138_services.up.sql); the integration suite
// pins that binding against the real constraint.
func StatusVocabulary() []string {
	out := make([]string, 0, len(transitions))
	for st := range transitions {
		out = append(out, st)
	}
	sort.Strings(out)
	return out
}

// transitions is the closed status machine. deleting is terminal (the
// reconciler removes the row after teardown + final backup).
var transitions = map[string][]string{
	"provisioning": {"ready", "failed", "deleting"}, // deleting = cancel-the-create
	"ready":        {"degraded", "suspended", "deleting"},
	"degraded":     {"ready", "failed", "deleting"},
	"failed":       {"provisioning", "deleting"}, // retry re-provisions
	"suspended":    {"ready", "deleting"},
	"deleting":     {},
}

// Observation is what the status machine decided to do with a cell's report.
//
// IT IS A TYPE, NOT A (string, bool) PAIR, and ADR-0014 is why. `to, _ :=
// ObservedStatus(...)` compiles, and dropping that second value silently
// re-introduces the defect US-3.3a round 12 was reverted for: the writeback
// advances observed_generation for a hop the machine has not finished, the row
// leaves the outstanding set, and nothing observes it again. A caller here
// cannot advance observation without having asked whether it converged.
//
// The zero value is deliberately useless: it reports no edge and not converged,
// so a caller that forgets to call ObservedStatus does nothing rather than
// something wrong.
type Observation struct {
	to        string
	edge      bool
	converged bool
}

// Edge is the transition to take, if any. ok=false means leave the status alone.
func (o Observation) Edge() (to string, ok bool) { return o.to, o.edge }

// Converged reports whether this report finishes the generation. FALSE means the
// machine needs another hop, so observed_generation must NOT advance — the row
// has to stay outstanding for the next tick. This is the same rule Kubernetes
// states for `status.observedGeneration`: it advances only when the controller
// has actually reconciled that generation, never merely because it looked.
func (o Observation) Converged() bool { return o.converged }

// settledStatuses are the statuses the platform is content to STOP WATCHING.
//
// `provisioning` is still working and `degraded` is impaired — a row that comes
// to rest on either has, by definition, not finished anything. Everything else
// is an end state: `ready` (healthy), `failed` (needs a human), and the two
// lifecycle holds, which never reach this table because a cell cannot report
// them (see reportableByCell).
//
// THIS IS THE WHOLE CONVERGENCE RULE. Four separate `converged: false` literals
// used to be written by hand, one per interesting hop, and every defect review
// found in this function was a hop where the hand-written flag was wrong or
// missing — `ready`+`degraded` converging on a billing state in one hop while
// `ready`+`failed` correctly took two, a transient `provisioning` finishing a
// generation mid-apply, and an unplaceable report finishing one at a stale
// status. Deriving the flag from the DESTINATION removes the class: there is no
// per-hop flag left to get wrong.
// It holds only the two END STATES a cell can actually report the row into.
// `suspended` and `deleting` are deliberately ABSENT rather than merely unused:
// `to` can only become one of them via an edge, step 2 refuses both as
// observations, and step 1 answers before any from-state could reach here — so
// entries for them would be dead weight a later sweep has to re-derive as
// equivalent mutants. Held rows settle in step 1, on their own rule.
var settledStatuses = map[string]bool{"ready": true, "failed": true}

// reportableByCell is the ADR-024 vocabulary a CELL can legitimately OBSERVE.
//
// `deleting` and `suspended` are things the control plane DOES to a service,
// never things a cell sees; the agent's statusFromPhase cannot produce either
// (it emits only ready / provisioning / failed). They are in the wire enum
// because that enum is the customer-facing ServiceStatus, shared with a field
// where they are meaningful.
var reportableByCell = map[string]bool{
	"provisioning": true, "ready": true, "degraded": true, "failed": true,
}

// ReportableByCell reports whether a cell may assert this status about a
// workload it is looking at. Neither `gone` nor "" is in the set — they are not
// claims about a workload's health (one says it is absent, the other says
// nothing at all) and both are answered ahead of this check, in step 1b.
//
// Exported because the HTTP handler enforces it too: reconcile's status route
// refuses a non-reportable status as a 422 before the request reaches the store.
// That route is the ONLY caller of Writeback, so it is the enforcement point;
// the arm below is a backstop for a direct caller, and it is deliberately NOT
// total — step 1 answers first, so on a `deleting` or `suspended` row a
// non-reportable status is accepted and converges. That is correct (those rows
// take no edge from any report, and making them 409 forever on a held status
// would be worse) but it does mean the backstop does not cover every from.
func ReportableByCell(observed string) bool { return reportableByCell[observed] }

// ObservedStatus maps a cell's OBSERVATION onto a status that is legal from the
// service's CURRENT one, and decides whether that finishes the generation.
//
// WHY THIS LIVES HERE AND NOT IN THE AGENT. `statusFromPhase` on the cell reads
// only the CNPG phase, so it answers identically whatever state the row is in —
// while the writeback asks "is this edge legal from svc.Status". ADR-024 allows
// `ready → {degraded, suspended, deleting}`, so a cluster that breaks while READY
// makes the agent report `failed`, Transition rejects it, observed_generation
// never advances, and the service is retried forever with nothing visible.
// Reachable normally: UpdateServiceShape bumps the generation for any status but
// `deleting`, and ListDesiredForCell has no status filter.
//
// US-3.3a round 12 put this in the AGENT and was reverted: a data-plane copy of
// a control-plane machine is a plane leak (ADR-0001 D9/A2.5), and it collapsed
// the agent's "never report a transient" guard on the way past. The control
// plane is the only place holding both `from` and the machine.
func ObservedStatus(from, observed string) Observation {
	// 1. HELD ROWS ANSWER FIRST. `deleting` is terminal (transitions["deleting"]
	// is empty) and a SUSPENDED service is never auto-resumed — `suspended →
	// ready` is a legal edge, so without this a converging agent that sees a
	// healthy cluster silently un-suspends the service and restarts its metering
	// span. Something held it; observing health is not consent to release it.
	// Neither ever takes an edge from a report.
	//
	// BUT THE HOLD DOES NOT FINISH THE GENERATION — only evidence that it was
	// APPLIED does, and the only such evidence a cell can give is `gone`.
	//
	// This is the guard in step 2 wearing its other face, and getting it wrong
	// here costs the same thing. `DeleteService` does BumpServiceGeneration
	// (gen → N+1) and only THEN Transition → `deleting`, so a deleting row is
	// OUTSTANDING by construction, and that is what redelivers the
	// `deleting:true` desired doc until the teardown is confirmed. Converging on
	// any report — a plain `ready` from a cell that has not torn anything down
	// yet — advances observed_generation, drops the row out of
	// ListDesiredForCell, and the cluster keeps running while DeleteService
	// answers "deletion already in progress" forever. Step 2 stops a cell
	// ASKING for a delete; this stops a cell ABANDONING one.
	//
	// `suspended` has no producer in the tree yet (nothing transitions a service
	// to it, and step 2 now refuses to let a cell report it), so its arm is a
	// stance rather than a measured requirement: a hold that was never observed
	// to take effect keeps the row outstanding, which is loud, rather than
	// settling it, which is invisible. Whatever implements suspend owns the
	// question of what evidence releases it.
	switch {
	case from == "deleting", from == "suspended":
		return Observation{to: from, edge: false, converged: observed == "gone"}
	}

	// 1b. NO STATUS REPORTED vs THE WORKLOAD IS GONE. These are different
	// answers and they used to be the same value: reconcile's handler collapsed
	// the wire's `gone` into "", so the machine could not tell "I converged this
	// generation and have nothing to say about its status" from "the thing you
	// asked me to run does not exist".
	//
	//   - "" is the observation-only ack. It converges: the agent applied the
	//     generation, the status is unchanged, and there is nothing to watch.
	//   - `gone` on a row that is NOT `deleting`/`suspended` (both answered
	//     above, where a missing workload is exactly what we asked for) means it
	//     VANISHED while desired still wants it alive. Settling that advances
	//     observed_generation on a service that no longer exists: the row leaves
	//     the outstanding set, the customer keeps seeing `ready`, metering keeps
	//     billing a `ready` span, and the agent — which re-creates from the
	//     desired doc and only ever sees rows ListDesiredForCell returns — never
	//     touches it again. Staying outstanding is what puts it back.
	//
	// The existing test for this path called an agent reporting `gone` for a
	// live service "a bug" and asserted only that the status was not mutated. It
	// was right about both; what it did not ask is what happens to the row
	// AFTERWARDS. Found by the convergence invariant, not by inspection — it is
	// the same defect as `ready` + `degraded` wearing the teardown path's clothes.
	// `gone` never converges here. "" is NOT short-circuited: it is the
	// observation-only ack, it carries no destination of its own, and it is
	// settled by the SAME rule as everything else in step 4 — so an ack on a row
	// that is still `provisioning` or `degraded` keeps the row outstanding
	// instead of parking it unwatched.
	if observed == "gone" {
		return Observation{to: from, edge: false, converged: false}
	}

	// 2. A CELL REPORTS WHAT IT OBSERVES — it does not issue lifecycle commands.
	//
	// Without this arm, CanTransition below accepts `deleting` and `suspended`
	// straight from `ready`, and one POST with the reconciler token bricks a
	// service permanently: the edge lands, but SetServiceStatus does NOT bump
	// the generation, so no `deleting:true` desired doc is ever produced and no
	// teardown runs; `deleting` has no outgoing edge; and DeleteService then
	// answers "deletion already in progress" forever — metering span closed,
	// workload still running. The reconciler secret is one shared value across a
	// configured cell list, so the blast radius is every service in every cell.
	//
	// This is the arm above, argued in the other direction: observing health is
	// not consent to resume, and observing anything is not consent to delete.
	//
	// NOT CONVERGED. Settling here would hand the same token a quieter attack —
	// POST a non-reportable status and observed_generation advances, dropping
	// the row out of ListDesiredForCell so it is never reconciled again.
	if observed != "" && !ReportableByCell(observed) {
		return Observation{to: from, edge: false, converged: false}
	}

	// 3. Choose the DESTINATION. Only legal edges, and only ones the machine can
	// justify from `from`.
	to, edge := from, false
	switch {
	// ADR-024 has no `ready → failed`. A cluster that broke while READY goes to
	// `degraded` — the legal edge and the semantically right answer. Without it
	// the answer would be "no change", and a broken database reported `ready`
	// forever is worse than the 409 loop this function exists to remove: no
	// writeback, no alert, indistinguishable from healthy.
	case from == "ready" && observed == "failed":
		to, edge = "degraded", true

	// ADR-024 has no `failed → ready` either, so a healthy cluster under a
	// failed row moves to `provisioning` and the NEXT tick lands `ready`.
	case from == "failed" && observed == "ready":
		to, edge = "provisioning", true

	case CanTransition(from, observed):
		to, edge = observed, true

	default:
		// No legal edge and no rule above: report NO CHANGE rather than
		// inventing one. This carries `from == ""` and a `from` outside the
		// machine entirely — neither can happen through the store (the services
		// CHECK constraint), and neither may be echoed back as a status, because
		// the same CHECK would refuse the UPDATE and turn bad input into a 500.
		// It also carries the in-vocabulary pairs with no edge between them,
		// such as `provisioning` + `degraded`, AND the steady-state report
		// (`observed == from`), which had its own arm until it was measured to
		// decide nothing: `transitions` has no self-edges, so
		// CanTransition(from, from) is always false and the arm produced the
		// identical `to = from, edge = false`. A self-edge added later would be
		// caught by TestEveryObservationReachesAFixedPointWithinTwoHops, which
		// fails on an edge to itself.
	}

	// 4. ONE RULE DECIDES CONVERGENCE, and it is about the DESTINATION, not the
	// hop: a generation is reconciled only when the row comes to rest on a
	// SETTLED status that the cell ACTUALLY REPORTED.
	//
	// Both halves are load-bearing:
	//
	//   - `settledStatuses[to]` keeps the row outstanding whenever it lands on
	//     `provisioning` (still working) or `degraded` (impaired). `degraded`
	//     BILLS (metering.IsBilling) and `degraded → failed` is the ONLY edge
	//     that emits a metering `close`, so a row that rests at `degraded` with
	//     nothing observing it again bills INDEFINITELY. That is true of the
	//     one-hop `ready` + `degraded` exactly as it is of the two-hop
	//     `ready` + `failed`, which is why this is not a per-hop decision.
	//
	//   - `to == observed` keeps it outstanding when we could not place the
	//     report — `ready` + `provisioning` (mid-apply, no legal edge) and
	//     `provisioning` + `degraded` both leave `to` at `from`, and settling
	//     there would advance observed_generation onto a status the cell never
	//     reported and drop the row out of ListDesiredForCell for good.
	//
	// This is the rule Kubernetes states for `status.observedGeneration`: it
	// advances when a generation is RECONCILED, not when it was merely looked at.
	//
	// WHAT THIS DOES NOT OWN, stated plainly because a wrong citation is worse
	// than none: a row that stays unconverged while genuinely stuck — a cluster
	// that reports `degraded` on every tick forever — has NO owner. It is
	// visible to the customer as `degraded`, which the bug this task fixes was
	// not, and `last_reconciled_at` is written on converged writebacks only and
	// has no reader anywhere in the repo. US-3.11 does NOT cover it (its ACs are
	// about the AGENT distinguishing an unrecoverable render error from a
	// transient one, not about a control-plane row that never advances).
	// Filed as US-3.12. Not reachable today: no CNPG phase maps to `degraded`
	// (unknown phases map to `failed`), but render's terminal() already accepts
	// `degraded`, so one phase-mapping change makes it live.
	//   - the `observed == ""` arm of the second half is the observation-only
	//     ack — "I applied this generation, I have nothing to say about status".
	//     It asserts nothing about where the row rests, so the FIRST half still
	//     decides: an ack on a `ready` row settles, an ack on a `provisioning` or
	//     `degraded` row does not.
	return Observation{to: to, edge: edge,
		converged: settledStatuses[to] && (to == observed || observed == "")}
}

// ObservedStatus is the method form, so *Service satisfies
// reconcile.Transitioner. The mapping is the package-level function above; this
// exists only because the reconciler holds a *Service, not a package.
//
// It returns the concrete Observation, which is the type reconcile.Transitioner
// names — see that interface for why the import points this way.
func (s *Service) ObservedStatus(from, observed string) Observation {
	return ObservedStatus(from, observed)
}

// CanTransition reports whether from → to is a legal edge.
func CanTransition(from, to string) bool {
	for _, t := range transitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

// desiredDoc builds the reconciler's desired-state document (US-1.3a) — what the
// cell-agent renders from (e1-substrate-design.md §2: product + intent + shape +
// lifecycle flags). Substrate names never appear here (D8); this is grammar
// only. `deleting` marks a teardown so the cell converges the service to gone.
// `namespace` is the env-derived cell namespace (ADR-0012).
func desiredDoc(product, intent, namespace string, quota billing.Quota, shape, scaling, override []byte, deleting bool) []byte {
	doc := map[string]any{"product": product}
	// THE PLAN'S PER-ENVIRONMENT ENVELOPE, resolved here and shipped as values.
	//
	// The cell-agent must not carry a copy of plans.json — same boundary as
	// pricing: a plan table in two modules is a plan table that drifts. The
	// control plane owns the mapping and the cell renders what it is given.
	//
	// KNOWN GAP, filed as US-3.3g: the doc is rebuilt when a SERVICE changes, not
	// when an org's PLAN changes. It is worse than staleness — the quota is
	// ENVIRONMENT-scoped but rendered from each SERVICE's doc, so after a plan
	// change the namespace carries whichever sibling converged last and the
	// ceiling OSCILLATES between plans until every doc is rewritten. The fix is a
	// plan-change hook that rewrites them in one transaction: a control-plane
	// concern, and not this task's.
	//
	// Services that predate this field are handled once, by migration
	// 20260823140000_service_quota_backfill, because tenancy.Render refuses a doc
	// with no envelope — they would otherwise never converge again.
	if quota != (billing.Quota{}) {
		doc["quota"] = map[string]string{
			"cpu": quota.CPU, "memory": quota.Memory, "storage": quota.Storage,
		}
	}
	if namespace != "" {
		doc["namespace"] = namespace // the cell renders here (env-derived, ADR-0012)
	}
	if intent != "" {
		doc["intent"] = intent
	}
	embed := func(key string, raw []byte) {
		if len(raw) > 0 {
			var v any
			if json.Unmarshal(raw, &v) == nil && v != nil {
				doc[key] = v
			}
		}
	}
	embed("shape", shape)
	embed("scaling", scaling)
	// override is the manual instance-pin {instances, reason, expires_at} — a
	// load-bearing capacity input (count), so the cell must render from it.
	embed("override", override)
	if deleting {
		doc["deleting"] = true
	}
	b, _ := json.Marshal(doc)
	return b
}

// refuseStorageShrink rejects a PATCH that lowers postgres storage below what
// the service already has provisioned.
//
// Both sides are RESOLVED. The stored shape already is (create persists the
// resolved form), so the merged one must be too — comparing a raw merged value
// against a resolved stored one refused PATCHes where nothing shrinks, e.g.
// `{"storage_gb":30}` on a `standard`, which CREATE accepts and resolves to 50.
// Resolving only raises values BELOW the floor, so a genuine reduction above it
// (200 -> 100) is still visible to this check.
func refuseStorageShrink(storedShape []byte, merged map[string]any) error {
	var stored map[string]any
	if json.Unmarshal(storedShape, &stored) != nil || stored == nil {
		return nil
	}
	prior, hadPrior := shapeGB(stored["storage_gb"])
	next, hasNext := shapeGB(merged["storage_gb"])
	// !hadPrior is a DELIBERATE pass, not an oversight — and it is now nearly
	// unreachable. UpdateService resolves the stored shape before merging, so any
	// postgres row whose size is in the catalog carries a number here. What is
	// left is a shape the catalog cannot resolve, which `storageForShape` refuses
	// outright: no volume was ever provisioned, so there is nothing to shrink.
	// Before that change this arm fired on every pre-US-3.7 verbatim-persist row
	// and silently accepted a downgrade to zero.
	if !hadPrior || !hasNext || next >= prior {
		return nil
	}
	// The remediation goes in the PROBLEM, not in the field Detail.
	// ValidationFailed's top-level remediation is "Fix the listed fields and
	// retry the request" — advice that cannot succeed here, because no value of
	// this field makes the request work. api-conventions requires each failure to
	// name a next action, and the next action is not a retry.
	p := problem.ValidationFailed([]problem.FieldError{{
		Field: "shape.storage_gb",
		// The wording is deliberate about WHICH claim is physical. QA found the
		// previous text ("a volume cannot shrink … so the 8 GB is still
		// provisioned") asserting a physical fact that is false below the
		// driver's 10Gi minimum volume: canon ships svc_jobs at storage_gb 4, and
		// 4 and 3 render the same 10Gi PVC, so nothing shrinks there. The REASON
		// the refusal is right at that size is billing, not geometry — the
		// recorded figure is what the invoice charges. State the rule, then the
		// mechanism behind it, and claim only what is true of both.
		Detail: fmt.Sprintf("cannot be reduced from %d to %d. Recorded storage only grows, "+
			"because the volume behind it cannot shrink — Kubernetes supports expansion only. "+
			"The %d GB stays provisioned and stays billed.", prior, next, prior),
	}})
	p.Remediation = "Create a new service at the smaller size and migrate the data; " +
		"retrying this request cannot succeed."
	return problemError{p: p}
}

// shapeGB reads a storage_gb that may have arrived as any JSON number shape.
func shapeGB(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), n == float64(int(n))
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	}
	return 0, false
}

// envelopeFor resolves an org's per-environment resource quota.
//
// Deny-by-default all the way down: an org whose plan is not in the table is a
// programming error (orgs.plan has a CHECK constraint listing exactly the four),
// and returning a zero Quota would render a ResourceQuota of "" that the API
// server rejects at apply — a config problem discovered on a cell.
func (s *Service) envelopeFor(ctx context.Context, orgID string) (billing.Quota, error) {
	org, err := s.q.GetOrg(ctx, orgID)
	if err != nil {
		return billing.Quota{}, fmt.Errorf("provisioning: org for quota envelope: %w", err)
	}
	return s.plans.Envelope(org.Plan)
}

// envelopeForService is the same, for the paths that hold a service rather than
// an org id.
func (s *Service) envelopeForService(ctx context.Context, serviceID string) (billing.Quota, error) {
	orgID, err := s.q.OrgForService(ctx, serviceID)
	if err != nil {
		return billing.Quota{}, fmt.Errorf("provisioning: org for service %s: %w", serviceID, err)
	}
	return s.envelopeFor(ctx, orgID)
}

// resolveNamespace derives the cell namespace for an environment.
//
// It is derived from the environment's ID, NOT from project/env NAMES. Names are
// unique only PER ORG (projects: UNIQUE (org_id, name)), so `proj--env` puts two
// different orgs' `api`/`prod` in the SAME namespace — sharing the tenant
// isolation boundary D7 defines (default-deny NetworkPolicy, ResourceQuota, and
// CNPG's generated `<cluster>-app` credential Secrets). Ids are globally unique
// AND immutable, so a project rename cannot orphan a running cluster either.
//
// Readability is preserved by keeping the env id recognizable (env_<hex> →
// env-<hex>); a human maps it back with one lookup, which is the right trade
// against cross-tenant namespace collision.
func (s *Service) resolveNamespace(ctx context.Context, envID string) (string, error) {
	if envID == "" {
		return "", fmt.Errorf("provisioning: cannot resolve a namespace without an environment id")
	}
	return NamespaceForEnv(envID), nil
}

// NamespaceForEnv maps an environment id to its RFC1123 namespace. Deterministic,
// immutable, and ≤63 chars by construction (env ids are short).
//
// EXPORTED SO THERE IS ONE DERIVATION. US-3.3b needs the namespace on the
// reconciler's poll, and US-3.3a already shipped a SECOND derivation of it
// agent-side — a label set to TrimPrefix(namespace, "env-"), which yielded
// `9f3c1a2b` for the id `env_9f3c1a2b` and named nothing the control plane knew.
// It was removed. Anything that needs this answer calls this function; nothing
// re-derives it, and the agent is told the namespace rather than computing it.
func NamespaceForEnv(envID string) string {
	ns := k8sNamespace(envID)
	if len(ns) > 63 {
		ns = strings.Trim(ns[:63], "-")
	}
	return ns
}

var nsInvalid = regexp.MustCompile(`[^a-z0-9-]`)

// k8sNamespace lowercases and dashes a name to an RFC1123 label segment.
func k8sNamespace(name string) string {
	n := nsInvalid.ReplaceAllString(strings.ToLower(name), "-")
	n = strings.Trim(n, "-")
	if n == "" {
		n = "x"
	}
	return n
}

// provisioningSteps is the C4 timeline, born with the row.
func provisioningSteps() []byte {
	steps := []map[string]string{
		{"step": "allocate", "status": "active"},
		{"step": "configure", "status": "pending"},
		{"step": "network/credentials", "status": "pending"},
		{"step": "first backup", "status": "pending"},
		{"step": "ready", "status": "pending"},
	}
	b, _ := json.Marshal(steps)
	return b
}

// CreateServiceInput is the decoded ServiceCreate.
type CreateServiceInput struct {
	Name       string
	Product    string
	Intent     string
	Shape      map[string]any
	EstimateID string
	ActorID    string
}

// CreateService enforces the estimate-before-provision law AT THE API LAYER:
// the estimate must accept (one-shot, env-fenced, live) AND contain a shape
// matching this service — what provisions is what was priced.
func (s *Service) CreateService(ctx context.Context, est *estimates.Service, env store.Environment, orgID string, in CreateServiceInput) (store.Service, error) {
	if in.Name == "" {
		return store.Service{}, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "name", Detail: "required"}})}
	}
	if in.EstimateID == "" {
		return store.Service{}, problemError{p: problem.ValidationFailed(
			[]problem.FieldError{{Field: "estimate_id", Detail: "required — nothing provisions without an accepted estimate (F2)"}})}
	}
	// NOTHING NEW GOES INTO AN ENVIRONMENT THAT IS BEING TORN DOWN. This mirrors
	// CreateProject's org check, and until US-3.3b it had no owner: scheduling an
	// environment's deletion did nothing at all, so creating a service into one
	// was merely odd.
	//
	// It is load-bearing now. DeleteEnvironment enforces "nothing live is in
	// here" at SCHEDULE time and nothing preserved it afterwards, so a service
	// created later sat inside a namespace the agent had been told to delete.
	// The reconciler's poll and its confirmation both fence on the same
	// condition, which turns that race into a refused confirmation rather than
	// data loss — but the honest fix is not to let it start.
	if env.DeletionScheduledAt.Valid {
		return store.Service{}, problemError{p: problem.Conflict(
			[]string{"this environment is scheduled for deletion"},
			"Create the service in another environment — this one's namespace is being torn down.")}
	}
	// Price line for THIS shape (also validates product/size before burning
	// the one-shot estimate).
	line, err := estimates.Price(estimates.ShapeInput{
		Product: in.Product, Intent: in.Intent, Name: in.Name, Shape: in.Shape,
	})
	if err != nil {
		var se estimates.ShapeError
		if errors.As(err, &se) {
			return store.Service{}, problemError{p: problem.ValidationFailed(
				[]problem.FieldError{{Field: se.Field, Detail: se.Detail}})}
		}
		return store.Service{}, err
	}

	// Coverage pre-check BEFORE burning the one-shot estimate (estimate rows
	// are immutable, so this is race-free): a mistyped create keeps the
	// estimate usable — better DX, same law.
	priced, pricedLines, err := est.PricedShapes(ctx, in.EstimateID)
	if err != nil {
		// A stored amount the money type refuses is not a server fault — answer
		// with the remediation (re-price) rather than a bare 500.
		if errors.Is(err, estimates.ErrStoredAmountUnrepresentable) {
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"this estimate holds a monthly amount outside the representable range"},
				"Create a new estimate for the same configuration and provision from that one — this estimate was priced before the platform bounded monetary arithmetic.")}
		}
		return store.Service{}, err
	}
	// The gate binds to the CONTRACTED CONFIGURATION, not to a price.
	//
	// Matching on (product, price) was substitutable: prices collide — a
	// postgres `dev` with 78 GB and a `standard` both come to 5800¢ — so a
	// caller could price one configuration and provision the other, in either
	// direction, and the gate agreed because the number agreed (US-3.7).
	//
	// estimates.Canonical resolves defaults, so the same configuration spelled
	// differently still matches; what it cannot do is let a DIFFERENT
	// configuration through because it happens to cost the same.
	want, err := estimates.Canonical(estimates.ShapeInput{
		Product: in.Product, Intent: in.Intent, Name: in.Name, Shape: in.Shape,
	})
	if err != nil {
		var se estimates.ShapeError
		if errors.As(err, &se) {
			return store.Service{}, problemError{p: problem.ValidationFailed(
				[]problem.FieldError{{Field: se.Field, Detail: se.Detail}})}
		}
		return store.Service{}, err
	}
	matched := false
	// A stored shape we cannot read must not condemn its SIBLINGS. Failing the
	// whole estimate on the first bad shape made the outcome depend on array
	// order: the same estimate would refuse or succeed depending on whether the
	// unreadable shape sat before or after the one being created.
	sawUnreadable := false
	for i, sh := range priced {
		got, cerr := estimates.Canonical(sh)
		if cerr != nil {
			// A stored shape we cannot canonicalize is an internal
			// inconsistency — but estimate rows are IMMUTABLE, so "retry" (the
			// 500 remediation) can never succeed. The actionable answer is the
			// same as any unusable estimate: get a fresh one. The cause is
			// logged rather than surfaced.
			slog.ErrorContext(ctx, "provisioning: stored estimate shape is not canonicalizable",
				"estimate", in.EstimateID, "index", i, "err", cerr)
			sawUnreadable = true
			continue
		}
		if got != want {
			continue
		}
		// The price the customer was SHOWN must still be the price they pay.
		// Comparing against a freshly computed price could only ever agree with
		// itself; the stored line is what was on their screen, so a pricing
		// table that moved under a live estimate is caught here.
		//
		// A missing line is an internal inconsistency, never a pass: skipping
		// the check would provision at the CURRENT price with no conflict —
		// silently failing open in a billing guard.
		if i >= len(pricedLines) {
			// KNOWN DEAD BRANCH, deliberately kept: `estimates.lines` is NOT
			// NULL and PriceAll preserves length, so this is unreachable today
			// and no test covers it (a `if false` here survives the suite —
			// stated so it is not mistaken for tested).
			//
			// Same reasoning as the un-canonicalisable shape above: fail
			// CLOSED, but with remediation the customer can act on. Estimate
			// rows are immutable, so "retry" could never succeed.
			slog.ErrorContext(ctx, "provisioning: estimate has fewer lines than shapes; cannot verify the price shown",
				"estimate", in.EstimateID, "shapes", len(priced), "lines", len(pricedLines))
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"this estimate can no longer be used"},
				"Create a fresh estimate for this environment and accept it — nothing provisions without one.")}
		}
		if pricedLines[i].MonthlyCents != line.MonthlyCents {
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"this shape has been repriced since the estimate was issued"},
				"Create a fresh estimate for this environment and accept it — the price you were shown is the price you pay.")}
		}
		matched = true
		break
	}
	if !matched {
		if sawUnreadable {
			// The requested shape matched nothing readable, and at least one
			// stored shape could not be read — so we cannot honestly say the
			// estimate does not cover it. Say what is actually true.
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"this estimate can no longer be used"},
				"Create a fresh estimate for this environment and accept it — nothing provisions without one.")}
		}
		return store.Service{}, problemError{p: problem.Conflict(
			[]string{"the estimate does not cover this shape"},
			"Estimate the exact shape you are creating, accept it, then create — the estimate IS the contract.")}
	}
	// T11.6 hard spend cap (F9): refuse BEFORE burning the one-shot estimate, so
	// a capped-out create leaves the estimate usable (raise the cap, retry).
	// Enforced here at the API layer — crossing the cap is impossible by
	// construction, never client-only advice.
	if err := s.enforceBudget(ctx, orgID, line.MonthlyCents); err != nil {
		return store.Service{}, err
	}
	if _, _, err := est.Accept(ctx, in.EstimateID, env.ID); err != nil {
		return store.Service{}, err
	}

	// Persist the RESOLVED shape, not the raw request map: what is stored — and
	// what the cell is handed — must be the configuration that was priced and
	// contracted, with defaults explicit rather than implied.
	resolvedShape, err := estimates.Resolve(estimates.ShapeInput{
		Product: in.Product, Intent: in.Intent, Name: in.Name, Shape: in.Shape,
	})
	if err != nil {
		return store.Service{}, err
	}
	// Resolved BEFORE the insert: an org whose plan has no envelope must fail the
	// create, not produce a service the cell can never give a quota.
	envelope, err := s.envelopeFor(ctx, orgID)
	if err != nil {
		return store.Service{}, err
	}
	shapeJSON, err := json.Marshal(resolvedShape)
	if err != nil {
		return store.Service{}, fmt.Errorf("provisioning: marshal shape: %w", err)
	}
	namespace := NamespaceForEnv(env.ID)
	row, err := s.q.InsertService(ctx, store.InsertServiceParams{
		ID: ids.New("svc"), EnvID: env.ID, Name: in.Name, Product: in.Product,
		Intent:               pgtype.Text{String: line.Intent, Valid: true},
		Shape:                shapeJSON,
		ProvisioningSteps:    provisioningSteps(),
		MonthlyEstimateCents: line.MonthlyCents.Int64(),
		EstimateID:           pgtype.Text{String: in.EstimateID, Valid: true},
		// US-1.3a/US-3.3: desired populated at creation with the resolved cell
		// namespace; the row is outstanding so the cell picks it up next poll.
		Desired: desiredDoc(in.Product, line.Intent, namespace, envelope, shapeJSON, nil, nil, false),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"a service with this name already exists in the environment"},
				"Pick a different name.")}
		}
		return store.Service{}, err
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "lifecycle", Via: "user", Actor: in.ActorID,
		Action: "service.created", Subject: row.ID,
		Detail: []byte(`{"name":` + strconv.Quote(in.Name) + `,"product":` + strconv.Quote(in.Product) + `,"estimate":` + strconv.Quote(in.EstimateID) + `}`),
	})
	return row, nil
}

// Transition moves a service along a legal edge, atomically (the SQL guard
// re-checks FROM), records the lifecycle event, and marks step timelines on
// the terminal provisioning edges. Illegal edges are conflicts, not crashes.
func (s *Service) Transition(ctx context.Context, svc store.Service, to, via, actor string, orgID string) (store.Service, error) {
	if !CanTransition(svc.Status, to) {
		return store.Service{}, problemError{p: problem.Conflict(
			[]string{"illegal status transition " + svc.Status + " → " + to},
			"Legal next states from "+svc.Status+": "+fmt.Sprint(transitions[svc.Status])+" (ADR-024).")}
	}
	var steps []byte
	if svc.Status == "provisioning" && to == "ready" {
		b, _ := json.Marshal([]map[string]string{
			{"step": "allocate", "status": "done"},
			{"step": "configure", "status": "done"},
			{"step": "network/credentials", "status": "done"},
			{"step": "first backup", "status": "done"},
			{"step": "ready", "status": "done"},
		})
		steps = b
	}
	row, err := s.q.SetServiceStatus(ctx, store.SetServiceStatusParams{
		ID: svc.ID, Status: svc.Status, Status_2: to, Steps: steps,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"service state changed concurrently"},
				"Re-read the service and retry the transition.")}
		}
		return store.Service{}, err
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "lifecycle", Via: via, Actor: actor,
		Action: "service." + to, Subject: svc.ID,
		Detail: []byte(`{"from":` + strconv.Quote(svc.Status) + `}`),
	})
	// D10: billing span edges — metering starts at ready, never before.
	if edge := metering.BillingEdge(svc.Status, to); edge != "" && s.meter != nil {
		if env, err := s.q.GetEnvironment(ctx, svc.EnvID); err == nil {
			s.meter.MustEmitSpan(ctx, metering.Tags{
				OrgID: orgID, ProjectID: env.ProjectID, EnvID: svc.EnvID, ServiceID: svc.ID,
			}, edge, svc.Product, svc.MonthlyEstimateCents)
		}
	}
	return row, nil
}

// ServiceOrg resolves service → org (404 for unknown ids — no probing).
func (s *Service) ServiceOrg(ctx context.Context, serviceID string) (store.Service, string, error) {
	svc, err := s.q.GetService(ctx, serviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Service{}, "", notFound("service")
	}
	if err != nil {
		return store.Service{}, "", err
	}
	orgID, err := s.q.OrgForService(ctx, serviceID)
	if err != nil {
		return store.Service{}, "", err
	}
	return svc, orgID, nil
}

// RunOverrideExpiry clears manual pins past their 24h expiry and rebuilds the
// desired doc so the cell converges back to the unpinned count.
//
// The expiry has to be swept, not merely filtered on read: `desired` is rebuilt
// only when someone edits the service, and generation is what makes the cell
// re-poll. A pin nobody touches again would otherwise render its instance count
// forever — which is what made "temporary" untrue and, with it, the argument
// for not charging for the capacity.
func (s *Service) RunOverrideExpiry(ctx context.Context, every time.Duration, log *slog.Logger) {
	sweep := func() {
		rows, err := s.q.ListExpiredOverrides(ctx)
		if err != nil {
			// ERROR, not warn: while this fails no pin anywhere expires, and
			// the only symptom is a log line.
			log.Error("override expiry sweep could not list expired pins; NO pin is expiring", "err", err)
			return
		}
		for _, row := range rows {
			if err := s.expireOverride(ctx, row); err != nil {
				log.Error("an expired pin could not be fully cleared", "service", row.ID, "err", err)
			} else {
				log.Info("manual override expired; converging back to the unpinned count", "service", row.ID)
			}
		}
	}
	sweep() // once at startup: a ticker does not fire immediately
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sweep()
		}
	}
}

// expireOverride clears one pin: unpinned price, unpinned desired doc, and the
// generation bump, in ONE statement — then re-cuts the billing span so the
// customer stops paying the pinned rate the moment the capacity goes away.
func (s *Service) expireOverride(ctx context.Context, row store.Service) error {
	var shape map[string]any
	_ = json.Unmarshal(row.Shape, &shape)
	base, err := estimates.Price(estimates.ShapeInput{
		Product: row.Product, Intent: row.Intent.String, Name: row.Name, Shape: shape,
	})
	if err != nil {
		return fmt.Errorf("base price: %w", err)
	}
	ns, err := s.resolveNamespace(ctx, row.EnvID)
	if err != nil {
		return fmt.Errorf("namespace: %w", err)
	}
	// Resolve everything the post-commit work needs BEFORE committing. A lookup
	// that fails after the UPDATE leaves the pin cleared and the span still
	// billing the pinned rate — with the row now unlistable (`override IS
	// NULL`), so no later sweep retries it and nothing detects it.
	orgID, err := s.q.OrgForService(ctx, row.ID)
	if err != nil {
		return fmt.Errorf("org lookup: %w", err)
	}
	sweepEnvelope, err := s.envelopeFor(ctx, orgID)
	if err != nil {
		return err
	}
	prior := row.MonthlyEstimateCents
	updated, err := s.q.ClearExpiredOverride(ctx, store.ClearExpiredOverrideParams{
		ID:                   row.ID,
		MonthlyEstimateCents: base.MonthlyCents.Int64(),
		Desired:              desiredDoc(row.Product, row.Intent.String, ns, sweepEnvelope, row.Shape, row.Scaling, nil, false),
		Generation:           row.Generation,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// The row moved under us — a concurrent edit. Not an error: the
			// next tick re-lists it if it is still pinned and still expired.
			return nil
		}
		return err
	}
	// The post-commit work runs on a context that CANNOT be cancelled by the
	// sweeper stopping. The clear has already committed; if shutdown cancels
	// between the commit and the spine write, the release is dropped and the
	// activity feed shows capacity pinned and never given back — with no error
	// anywhere, since `record` discards its own. Same reasoning as US-3.6's
	// idempotency recorder, which hit this as a live defect.
	ctx = context.WithoutCancel(ctx)
	s.repriceSpan(ctx, orgID, updated, prior, updated.MonthlyEstimateCents)
	// The expiry is a state change like any other, so it goes to the SPINE, not
	// only to a log line. Applying a pin records `service.updated` carrying the
	// operator's reason (below); without this the activity feed shows capacity
	// being pinned and never released, and the only account of the release is a
	// log the customer cannot see. `Via: "system"` because no actor asked for it
	// — the clock did. The expired pin travels in the detail: it is the answer to
	// "what was released, and what reason had been given for it".
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "scale", Via: "system", Actor: "system",
		Action: "service.updated", Subject: updated.ID,
		Detail: []byte(`{"override_expired":` + string(row.Override) + `}`),
	})
	return nil
}

// repriceSpan makes a mid-life rate change reach the INVOICE.
//
// Billing derives solely from `usage_events.rate_cents`, which is snapshotted
// when a span opens and which the rollup multiplies by every second of that
// span. So changing `services.monthly_estimate_cents` moves the forward-looking
// cap and the API response and NOTHING ELSE: the open span keeps billing at the
// rate it opened with. A pin that raised capacity 9x was billed at 1x.
//
// A rate change is therefore a close-at-the-old-rate plus an open-at-the-new —
// the shape the rollup's open/close pairing already expects. No span is open
// outside a billing status, so there is nothing to re-cut there.
func (s *Service) repriceSpan(ctx context.Context, orgID string, svc store.Service, oldCents, newCents int64) {
	if s.meter == nil || oldCents == newCents || !metering.IsBilling(svc.Status) {
		return
	}
	env, err := s.q.GetEnvironment(ctx, svc.EnvID)
	if err != nil {
		slog.ErrorContext(ctx, "reprice: environment lookup failed; the span keeps the old rate and the invoice will be wrong",
			"service", svc.ID, "err", err)
		return
	}
	tags := metering.Tags{OrgID: orgID, ProjectID: env.ProjectID, EnvID: svc.EnvID, ServiceID: svc.ID}
	s.meter.MustEmitSpan(ctx, tags, "close", svc.Product, oldCents)
	s.meter.MustEmitSpan(ctx, tags, "open", svc.Product, newCents)
}

// overrideInstances reports the pinned instance count of a LIVE override.
//
// D22 makes the pin temporary: it carries a reason and auto-expires in 24h. The
// expiry was being written and never read — nothing anywhere consulted
// `expires_at` — so a "temporary" pin was permanent, which is also what removed
// the only argument for not charging for the capacity it provisions.
//
// Returns (0, false) for an absent, malformed, or expired override.
//
// Two of the checks below are EQUIVALENT MUTANTS — `len(raw) == 0` and
// `ExpiresAt == ""` can both be deleted with no observable change, because the
// Unmarshal and the time.Parse under them already fail on those inputs. No test
// can distinguish them and none pretends to. They stay because they say what
// the rule IS ("unset is not forever"), rather than leaving it as a side effect
// of a parser erroring. That equivalence is CONDITIONAL on the parse staying
// strict: relax time.Parse to accept more layouts, or make an empty expiry mean
// "no constraint", and `ExpiresAt == ""` becomes load-bearing again — while
// mutation testing has it filed as equivalent and will not re-flag it.
func overrideInstances(raw []byte, now time.Time) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var o struct {
		Instances int    `json:"instances"`
		ExpiresAt string `json:"expires_at"`
	}
	// The `o.Instances < 1` half is now UNREACHABLE from production: the handler
	// refuses `override.instances < 1` with a 422 before anything reaches here
	// (services_http.go). It stays as the service layer's own floor — this
	// function is also the read side for rows planted by a migration or a
	// support script, which the handler never sees — and it is covered by
	// TestOverrideLiveness. Noted because this file records which arms are
	// reachable and why, and an unmarked one is how a live guard gets filed as
	// decoration.
	if err := json.Unmarshal(raw, &o); err != nil || o.Instances < 1 {
		return 0, false
	}
	if o.ExpiresAt == "" {
		// A pin with no expiry is not a temporary pin. Refuse to honour it
		// rather than treat "unset" as "forever".
		return 0, false
	}
	exp, err := time.Parse(time.RFC3339, o.ExpiresAt)
	if err != nil || !now.Before(exp) {
		return 0, false
	}
	return o.Instances, true
}

// UpdateService — shape/scaling are desired-state edits (repriced); the
// manual override requires a reason and auto-expires in 24h (D22).
func (s *Service) UpdateService(ctx context.Context, svc store.Service, orgID, actorID string, shape map[string]any, scaling, override []byte) (store.Service, error) {
	// A deleting service must not be edited: US-1.3a made desired load-bearing,
	// so an edit would rewrite desired with deleting=false and re-outstand the
	// row, cancelling an in-flight teardown. Reject it (before US-1.3a desired
	// was inert '{}', so this clobber is newly reachable).
	if svc.Status == "deleting" {
		return store.Service{}, problemError{p: problem.Conflict(
			[]string{"the service is being deleted"},
			"Wait for deletion to complete; a deleting service cannot be edited.")}
	}
	params := store.UpdateServiceShapeParams{ID: svc.ID}
	if shape != nil {
		// merge over the existing shape: PATCH semantics, absent keys survive
		var current map[string]any
		_ = json.Unmarshal(svc.Shape, &current)
		if current == nil {
			current = map[string]any{}
		}
		// RESOLVE THE STORED SHAPE BEFORE MERGING — this is what makes the
		// storage ratchet structural rather than a property of how the row
		// happened to be written.
		//
		// "Absent keys survive" only retains what is THERE. A row stored as
		// `{"size":"standard"}` with no storage_gb — the verbatim-persist shape
		// that predates US-3.7, and the one the ratchet migration cannot reach
		// because its WHERE requires a catalogued `size` — loses the 50 GB it
		// actually has the moment a PATCH changes the size: the merge yields
		// `{"size":"dev"}`, which resolves to 0, and refuseStorageShrink's
		// no-prior-value arm lets it through. Measured on that row: ACCEPTED,
		// priced 1900 against the ruled 4400, desired 0, and the cell renders a
		// 10Gi PVC against a 50Gi volume the CSI driver will refuse.
		//
		// Resolving here recovers the floor from the catalog, so the merge has a
		// number to preserve and every downstream comparison sees the same
		// value. It is a no-op for a row create already resolved, which is all of
		// them since US-3.7 — the point is that the guard no longer DEPENDS on
		// that being true of history.
		if resolvedCurrent, err := estimates.Resolve(estimates.ShapeInput{
			Product: svc.Product, Intent: svc.Intent.String, Name: svc.Name, Shape: current,
		}); err == nil {
			current = resolvedCurrent
		}
		// A stored shape that does NOT resolve is left raw on purpose: it names a
		// size the catalog does not have, so `cnpg.storageForShape` refuses it and
		// no volume was ever provisioned for it. There is nothing to ratchet, and
		// the Resolve of the MERGED shape below is what reports the real problem.
		for k, v := range shape {
			current[k] = v
		}
		// Persist the RESOLVED merged shape, exactly as create does. Storing a
		// raw map here and a resolved one there would mean the same
		// configuration is spelled two ways depending on how the service got
		// there — and the identity that gates provisioning is computed from the
		// resolved form.
		resolvedMerged, err := estimates.Resolve(estimates.ShapeInput{Product: svc.Product, Name: svc.Name, Shape: current})
		if err != nil {
			// Resolve is now the FIRST validator of the merged shape — the
			// Price call that used to be here owned the ShapeError → 422
			// conversion as a second job nobody had written down. Without this,
			// a client typo (`{"bogus":1}`, `{"size":123}`) became a 500 with
			// an event id and "contact support" instead of the field that is
			// wrong, while the same input still returned 422 on
			// POST /v1/estimates — one class of error, two answers.
			var se estimates.ShapeError
			if errors.As(err, &se) {
				return store.Service{}, problemError{p: problem.ValidationFailed(
					[]problem.FieldError{{Field: se.Field, Detail: se.Detail}})}
			}
			return store.Service{}, err
		}
		// AFTER resolve, and against the RESOLVED merged shape. Comparing the raw
		// merged value was wrong in one direction: the STORED shape is already
		// resolved (create persists the resolved form), so `PATCH
		// {"storage_gb":30}` on a `standard` compared 30 against 50 and 422'd —
		// while the identical body on CREATE resolves to 50 and is accepted. That
		// is exactly the invariant this branch names as load-bearing: "the same
		// configuration spelled with its defaults written out must not be
		// refused". Resolving only RAISES values below the floor, so a real
		// reduction above it (200 -> 100) is still visible on both sides.
		//
		// A PVC CANNOT SHRINK: Kubernetes supports expansion only and rejects a
		// request below `.status.capacity`. Without this, `PATCH
		// {"storage_gb":20}` on a 200 GB service drops the bill for storage the
		// cluster is still carrying AND makes the driver render a 20Gi PVC the CSI
		// driver refuses, leaving the row outstanding forever with nothing written
		// back. Refused rather than silently floored — this is NOT a pricing
		// decision; nobody's bill moves.
		if err := refuseStorageShrink(svc.Shape, resolvedMerged); err != nil {
			return store.Service{}, err
		}
		merged, err := json.Marshal(resolvedMerged)
		if err != nil {
			return store.Service{}, err
		}
		params.Shape = merged
	}
	params.Scaling = scaling

	// Price the EFFECTIVE post-edit configuration, unconditionally.
	//
	// Doing this only inside the shape branch and the live-pin branch left a
	// hole the moment "any PATCH clears the pin" became real: `PATCH {"scaling":…}`
	// or `PATCH {}` released the capacity but `monthly_estimate_cents` kept the
	// PINNED rate — and the row was then unsweepable (`override IS NULL`), so
	// nothing ever restored it. The customer paid the pinned rate forever for
	// capacity that was gone, and the phantom charged against their hard cap.
	effShapeRaw := svc.Shape
	if params.Shape != nil {
		effShapeRaw = params.Shape
	}
	var effShapeMap map[string]any
	_ = json.Unmarshal(effShapeRaw, &effShapeMap)
	priceIn := estimates.ShapeInput{
		Product: svc.Product, Intent: svc.Intent.String, Name: svc.Name, Shape: effShapeMap,
	}
	var effLine estimates.Line
	var priceErr error
	if pinned, live := overrideInstances(override, time.Now()); live {
		// Founder ruling (2026-07-27): pinned capacity is METERED, so the pin
		// is priced through the engine and refused when the catalog cannot
		// price it.
		effLine, priceErr = estimates.PriceWithInstances(priceIn, pinned)
	} else {
		// No live pin: the effective price is the configuration's base — which
		// is what RELEASES a pinned rate, whether or not the shape changed.
		effLine, priceErr = estimates.Price(priceIn)
	}
	if priceErr != nil {
		var se estimates.ShapeError
		if errors.As(priceErr, &se) {
			return store.Service{}, problemError{p: problem.ValidationFailed(
				[]problem.FieldError{{Field: se.Field, Detail: se.Detail}})}
		}
		return store.Service{}, priceErr
	}
	// T11.6 hard cap: any increase in committed monthly spend — a scale-up or a
	// pin — must clear the cap exactly like a create. Only the increase is
	// projected, since the run-rate already includes this service's old cost;
	// a decrease is always allowed.
	// The STORED price is re-validated on the way in: a row left out of range by
	// a past wrap must not become the baseline an increase is measured against.
	// Sub is reached only when the new price is strictly greater, so it cannot
	// go negative.
	priorAmt, priorErr := money.FromInt(svc.MonthlyEstimateCents)
	if priorErr != nil {
		return store.Service{}, problemError{p: problem.Conflict(
			[]string{"this service's stored monthly estimate is not a valid amount"},
			"Contact support: the stored price is outside the representable range, so a change cannot be priced against it.")}
	}
	if effLine.MonthlyCents.GreaterThan(priorAmt) {
		delta, err := effLine.MonthlyCents.Sub(priorAmt)
		if err != nil {
			return store.Service{}, err
		}
		if err := s.enforceBudget(ctx, orgID, delta); err != nil {
			return store.Service{}, err
		}
	}
	params.MonthlyEstimateCents = pgtype.Int8{Int64: effLine.MonthlyCents.Int64(), Valid: true}

	params.Override = override
	// US-1.3a: rebuild desired from the effective post-edit state and let the
	// query bump generation, so the cell re-reconciles. Effective shape/scaling
	// = the edit if present, else the current row.
	effShape := svc.Shape
	if params.Shape != nil {
		effShape = params.Shape
	}
	effScaling := svc.Scaling
	if scaling != nil {
		effScaling = scaling
	}
	// The desired doc carries EXACTLY what the column gets. UpdateServiceShape
	// sets `override = sqlc.narg('override')` unconditionally, so a PATCH with
	// no override key NULLs the column — and keeping svc.Override in the doc
	// left the pin rendering forever, unsweepable (the sweep matches only
	// `override IS NOT NULL`) and un-un-pinnable, since that PATCH is the only
	// way a customer clears one.
	effOverride := override
	// Never ship an EXPIRED pin to the cell. Without this an override written
	// once keeps its instance count forever, because nothing else consults
	// expires_at and the doc is only rebuilt when someone edits the service.
	if _, live := overrideInstances(effOverride, time.Now()); !live {
		effOverride = nil
	}
	ns, err := s.resolveNamespace(ctx, svc.EnvID)
	if err != nil {
		return store.Service{}, err
	}
	updEnvelope, err := s.envelopeForService(ctx, svc.ID)
	if err != nil {
		return store.Service{}, err
	}
	params.Desired = desiredDoc(svc.Product, svc.Intent.String, ns, updEnvelope, effShape, effScaling, effOverride, false)
	priorCents := svc.MonthlyEstimateCents
	params.Generation = svc.Generation
	row, err := s.q.UpdateServiceShape(ctx, params)
	if err != nil {
		// Zero rows means the row moved under us: either a delete raced in
		// (the `status <> 'deleting'` fence) or a concurrent edit did (the
		// generation fence). Both are "re-read and retry" — writing anyway
		// would overwrite the other edit's shape, doc and PRICE from a stale
		// read, leaving the column, the cell and the invoice each holding a
		// different answer.
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Service{}, problemError{p: problem.Conflict(
				[]string{"the service changed while this request was in flight"},
				"Re-read the service and retry; it was deleted or edited concurrently.")}
		}
		return store.Service{}, err
	}
	// Post-commit, so the context must not be cancellable. The row is already
	// written; a client disconnecting between here and the two calls below would
	// leave the price changed with the span still billing the OLD rate — the
	// precise defect repriceSpan exists to prevent — and the pin's reason would
	// never reach the spine, with no error anywhere, since `record` discards its
	// own and MustEmitSpan only logs. expireOverride's structurally identical
	// block got this treatment first; this one is ten lines away and was missed.
	//
	// KNOWN UNCOVERED: deleting this line survives mutation. Forcing it needs a
	// client disconnect in the window between the commit and the two calls
	// below, which no deterministic test can place — an already-cancelled
	// context fails the write instead, and a short deadline is a race. Recorded
	// rather than claimed, and the same is true of expireOverride's copy.
	ctx = context.WithoutCancel(ctx)
	// The rate the customer PAYS follows the row. Without this a pin (or a
	// scale-up) changes monthly_estimate_cents while the open span keeps
	// billing at the pre-change rate — nine instances provisioned, one billed.
	s.repriceSpan(ctx, orgID, row, priorCents, row.MonthlyEstimateCents)
	// The pin's REASON is the whole audit value of an affordance that
	// provisions capacity outside the normal estimate path, so it reaches the
	// spine rather than being recorded as a bare "service.updated".
	detail := []byte(`{}`)
	if len(override) > 0 {
		detail = []byte(`{"override":` + string(override) + `}`)
	}
	s.record(ctx, events.Input{
		OrgID: orgID, Kind: "scale", Via: "user", Actor: actorID,
		Action: "service.updated", Subject: svc.ID, Detail: detail,
	})
	return row, nil
}

// DeleteService — desired state → deleting (202). The final backup + actual
// teardown are the driver's job (T3.4/US-3.5); dependents (bindings, T3.6)
// join the 409 check when they exist.
func (s *Service) DeleteService(ctx context.Context, svc store.Service, orgID, actorID string) error {
	if svc.Status == "deleting" {
		return problemError{p: problem.Conflict([]string{"deletion already in progress"},
			"The service is already deleting; the final backup will be recorded.")}
	}
	// U6: dependents that will knowingly break are NAMED, all of them.
	deps, err := s.q.ActiveBindingsToTarget(ctx, pgtype.Text{String: svc.ID, Valid: true})
	if err != nil {
		return err
	}
	if len(deps) > 0 {
		reasons := make([]string, 0, len(deps))
		for _, d := range deps {
			reasons = append(reasons, "service "+d.SourceName+" binds to this service ("+d.ID+")")
		}
		return problemError{p: problem.Conflict(reasons,
			"Unbind the listed services first — deleting would knowingly break them (U6).")}
	}
	// US-1.3a: write the deleting desired doc + bump generation (BumpServiceGeneration)
	// so the service becomes outstanding and the cell converges the teardown,
	// THEN transition status to deleting. Two writes, not one transaction (the
	// atomicity hardening is a carried finding). A crash BETWEEN them is not
	// poll-self-healing — the cell would converge the teardown and report `gone`,
	// which takes no edge, so status would stay at its pre-delete value while the
	// desired doc says deleting. Since US-3.3h that report does not CONVERGE
	// either (status is not yet `deleting`, so `gone` means "it vanished while
	// desired still wants it"), which leaves the row outstanding and the
	// idempotent teardown re-issued each tick instead of dropping silently out of
	// the outstanding set. It IS retry-recoverable: status is not yet `deleting`,
	// so a second DeleteService passes the guard above and completes the
	// transition. Ordered desired-first deliberately: the reverse
	// (status-first) would strand a row that the guard then refuses to retry.
	dns, err := s.resolveNamespace(ctx, svc.EnvID)
	if err != nil {
		return err
	}
	// NO ENVELOPE ON THE TEARDOWN DOC, deliberately. Converge's deleting branch
	// returns before any tenancy object is rendered, so the value would never be
	// read — and resolving it would give deletion a new way to fail (a missing
	// org row, or a plan absent from the table) on a capability plans.json lists
	// under `never_gated: self_deletion`. Plans gate capabilities, never safety.
	del := desiredDoc(svc.Product, svc.Intent.String, dns, billing.Quota{}, svc.Shape, svc.Scaling, svc.Override, true)
	if _, err := s.q.BumpServiceGeneration(ctx, store.BumpServiceGenerationParams{ID: svc.ID, Desired: del}); err != nil {
		return err
	}
	_, err = s.Transition(ctx, svc, "deleting", "user", actorID, orgID)
	return err
}

func (s *Service) ListServices(ctx context.Context, envID string) ([]store.Service, error) {
	return s.q.ListServicesForEnv(ctx, envID)
}
