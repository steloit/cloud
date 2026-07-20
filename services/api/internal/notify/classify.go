package notify

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

// classification is one action → bell row rule.
type classification struct {
	kind string
	// title is the frame's copy. Placeholders are filled by the projection;
	// the invariant part must appear verbatim in the gallery.
	title string
	// fragment is the contiguous, frame-verbatim substring asserted against the
	// N1/N2 region. It must survive tag-stripping — the gallery interleaves
	// markup inside titles (`<b>db-reports</b> is ready — …`), so a fragment
	// spanning a tag boundary would never match.
	fragment string
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
		kind:     KindDeploy,
		title:    "Deploy #%s to %s by %s",
		fragment: "Deploy #",
	},
}

// Classify reports whether a spine action is notification-worthy and, if so,
// the kind and title template the bell row carries.
//
// When ok is false both kind and title are empty — never a half-populated
// result a caller could mistake for a usable title.
func Classify(action string) (kind, title string, ok bool) {
	c, found := classifications[action]
	if !found {
		return "", "", false
	}
	return c.kind, c.title, true
}
