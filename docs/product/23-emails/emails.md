# Transactional emails

The one user-facing surface the frames don't cover. Voice: `17-brand/voice.md` applies in full — an email is **an event with a deep link**; it earns its interruption or it isn't sent (the N-series severity discipline, extended to the inbox). Routing is the human's (P6 delivery matrix); firing is the org's; quiet hours never gate escalation paging.

## Rules

1. **One event, one email, one action.** Subject states the fact; body: what happened → the numbers → one primary link into the story (the N2 detail pane's deep-link, reused). No digests of unrelated things; no marketing in transactional mail, ever.
2. **Subject grammar:** `[<org>/<project>] <fact>` — mono-style precision, no urgency theater. `[acme/ecommerce] api p95 alert firing — 812 ms`, never "⚠️ URGENT: Action required!".
3. **The contract travels.** Emails about money or lifecycle state the same contract the console prints (dunning timeline, quota math, wind-down) — same numbers, same words. If banner and email disagree, one of them is a defect.
4. **Text-first**, brand-light: wordmark, then content; Helvetica/Courier fallbacks per brand.md; the lockup SVG never rendered in fallback fonts. Every email foots with *why you received this* + the per-type routing link (P6) — mute the type, never beg to stay.
5. **Security emails cannot be muted** (new session, password change, token created, member removed) and always include the "that wasn't you? → Revoke & secure" bridge (P4).

## Inventory (each maps to an Event kind; nothing else sends mail)

| Trigger | Subject (grammar) | Body carries |
|---|---|---|
| Invite (F1) | `<inviter> invited you to <org> as <role>` | role explained by consequences (A6's list) · 7-day expiry stated · accept link bound to this address |
| Invite expired → renewed | `Your invite to <org> was renewed` | new link; inviter notified line |
| Quota 80% (B8) | `[org] egress at 87 of 100 GB` | the math (~$1.62 at current pace) · calculator framing, "do nothing" valid · link to Quotas |
| Alert firing (P6 routing) | `[org/project] <rule> firing — <value>` | evidence sentence · mini context (deploy marker) · Open in Observe · silence link (logged) |
| Dunning day 0 / 7 / 21 | `[org] payment failed — <contract stage>` | the full timeline with *you are here* · what's paused vs untouched · update-card link · day-90 rule verbatim |
| Payment cleared | `[org] payment received — everything restored` | what resumed, instantly |
| Trial ending (B9) | `[org] Pro trial ends <date>` | end-state contract restated (lapse to Free, nothing deleted) |
| Plan changed / cancelled (B11/12) | `[org] plan: <from> → <to>` | proration math or wind-down contract echo |
| Cert issued / domain verified (U5) | `[project] <domain> is live` | cert auto-renews line |
| Backup failure | `[org/project] backup failed for <service>` | what's still protected (last good backup) · next retry · never silent |
| Security events (P3/P4) | `New session on <device>` etc. | that-wasn't-you bridge; unmutable |
| Provisioning failed (C4) | `[project] <service> failed to provision` | step it failed at · **never billed** stated · audit event id |

Deploys and routine lifecycle events are **in-app only** by default (read-optional context, never noise-by-default). Proposals/insights never email — they wait in the product (AI proposals never animate in; they don't chase you to your inbox either).

## Skeleton (all templates)

```
Subject: [acme/ecommerce] api p95 alert firing — 812 ms
Preheader: since 14:02 · deploy #142 preceded it

<wordmark>

api p95 crossed 800 ms at 14:02 (now 812 ms).
Deploy #142 shipped at 13:58 — the charts carry the marker.

  [ Open in Observe ]

Silence for 24 h (logged) · Alert settings

— Steloit
You receive production alerts in-app and by email · change routing (P6)
```
