package notify

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/steloit/cloud/services/api/internal/identity/store"
)

// Classification of a spine action into the notification it becomes on the bell.
//
// Two rules govern this file and neither is negotiable:
//
//  1. The KIND vocabulary is closed. The founder ruled seven kinds on
//     2026-07-20; an eighth is a new founder decision, never an engineering
//     one (AGENTS.md §5 rule 10).
//  2. Every TITLE is frame-verbatim. Titles come from the N1/N2 frames in
//     docs/product/00-sources/Steloit-Console-Screens.html and nowhere else.
//     An action with no frame row classifies as NOT notification-worthy — we
//     stay silent rather than invent copy. TestClassifyTitlesMatchFrameSource
//     enforces this mechanically against the gallery source.
//
// Silence is the default and the common case: the badge is only honest because
// most spine events never reach the bell.

// The seven ruled notification kinds. Do not add an eighth.
const (
	KindAlert     = "alert"
	KindProposal  = "proposal"
	KindApproval  = "approval"
	KindDeploy    = "deploy"
	KindLifecycle = "lifecycle"
	KindBilling   = "billing"
	KindSecurity  = "security"
)

// renderer resolves the presentation data a frame title needs and renders it.
//
// Founder ruling 2026-07-20 (Option A): render ONCE during projection and
// persist the rendered title, because a notification is a historical fact — a
// later rename must not rewrite what a user was told. Two constraints bind
// every renderer:
//
//   - Deterministic, LOCAL reads only. No network, nothing asynchronous; the
//     projection runs inside a transaction and must never hold it across I/O.
//   - Never invent copy. If the presentation data is missing, return ok=false
//     and the projection stays silent. A rendered-but-wrong title is worse than
//     no bell row, and undocumented copy is not ours to author.
type renderer func(ctx context.Context, s Store, ev store.Event) (title string, ok bool, err error)

// classification is one action → bell row rule.
type classification struct {
	kind string
	// title is the frame's copy, as a format string. The invariant part must
	// appear verbatim in the gallery.
	title string
	// fragment is the contiguous, frame-verbatim substring asserted against the
	// N1/N2 region. It must survive tag-stripping — the gallery interleaves
	// markup inside titles (`<b>db-reports</b> is ready — …`), so a fragment
	// spanning a tag boundary would never match.
	fragment string
	// render resolves this classification's presentation data.
	render renderer
}

// deployCreatedTitle is N1's copy as a format string. Declared as a const so the
// renderer does not read the table that holds it (initialization cycle).
const deployCreatedTitle = "Deploy #%d to %s by %s"

// renderDeployCreated builds N1's `Deploy #142 to production by priya`.
//
// The spine carries the number (Detail) but only IDs for the env and actor, so
// both names are resolved here — one local query, inside the projection tx.
// This does not denormalize the spine: the resolved title lands on the
// notifications row, which is a projection, not the ledger.
func renderDeployCreated(ctx context.Context, s Store, ev store.Event) (string, bool, error) {
	row, err := s.GetDeployNotificationContext(ctx, store.GetDeployNotificationContextParams{
		DeploymentID: ev.Subject,
		ActorID:      ev.Actor,
		OrgID:        ev.OrgID, // org-fenced: never render another tenant's env name
	})
	if err != nil {
		// A deployment deleted before projection is not an error worth failing
		// the batch over — it is simply unrenderable.
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	// users.name is contract-required at signup but the column defaults to '',
	// so an empty name is reachable. We do NOT substitute the email local-part
	// or any generated stand-in — that would be undocumented copy. Unrenderable
	// means silent, and the gap is tracked as a follow-up.
	if row.ActorName == "" || row.EnvName == "" {
		return "", false, nil
	}
	return fmt.Sprintf(deployCreatedTitle, row.Number, row.EnvName, row.ActorName), true, nil
}

// classifications is the whole table. It is deliberately short.
//
// N1 shows seven rows, but six of them describe planes that do not exist yet,
// so there is no spine action to key them off:
//
//	approval request      — approval threads are unspecced (ledger §2c)
//	assistant proposal    — AI proposal surface (E13) emits no spine action
//	alert (p95 crossed)   — alert rules (E12) do not evaluate yet
//	dead-letter queue     — no queue runtime (E1)
//	"<svc> is ready"      — the ready transition is E1 provisioning
//	certificate renewed   — domain/cert management not built
//
// Each becomes one line here the moment its plane emits an action AND its frame
// row supplies the copy. Adding one before both hold means inventing a title,
// which is the exact failure recorded at spec-change-proposals.md:204.
var classifications = map[string]classification{
	// N1 "Today": `Deploy #142 to production by priya`.
	"deploy.created": {
		kind:  KindDeploy,
		title: deployCreatedTitle,
		// The FULL static shape, not just a prefix: this pins the verb order
		// ("to <env> by <actor>") to the frame, so swapping the format args fails.
		fragment: "Deploy #142 to production by ",
		render:   renderDeployCreated,
	},
}

// Classify reports whether a spine action is notification-worthy and, if so,
// the kind and the title FORMAT STRING (e.g. "Deploy #%d to %s by %s") — not
// display copy. It is unexported deliberately: a caller that persisted the
// returned title verbatim would ship "Deploy #%!d(MISSING)" to a user. Render
// through a classification's renderer instead.
//
// When ok is false both kind and title are empty — never a half-populated
// result a caller could mistake for a usable title.
func classifyAction(action string) (kind, title string, ok bool) {
	c, found := classifications[action]
	if !found {
		return "", "", false
	}
	return c.kind, c.title, true
}

// classify returns the whole rule, renderer included. Internal because the
// renderer is an implementation detail of the projection.
func classify(action string) (classification, bool) {
	c, found := classifications[action]
	return c, found
}
