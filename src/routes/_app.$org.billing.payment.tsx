import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Banner } from "@/design-system/banner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Dot, Pill } from "@/design-system/pill";
import {
  planLabel,
  useCancelPlan,
  useChangePlan,
  usePaymentMethods,
  useSubscription,
} from "@/features/billing/hooks";
import { useOrgs } from "@/features/org/hooks";
import { errorMessage, ProblemError, type Subscription } from "@/lib/api";
import { CANON_NOW } from "@/lib/canon/now";

/**
 * B4 · Payment & plan (Acme, status current) plus the B9/B11/B12 lifecycle
 * states for Borealis, driven by Subscription.status. B10 dunning and B11's
 * dedicated confirm page are states the canon-mode subscription doesn't
 * enter — implemented data-driven off Subscription.status where reachable;
 * the frames' Mar/Apr dates are reconciled to the fixtures' July cycle.
 */

const PLAN_PRICE: Record<string, string> = {
  free: "$0 / mo",
  pro: "$29 / mo",
  business: "$99 / mo",
  enterprise: "custom",
};

function fmtAnchorDate(iso: string): string {
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    timeZone: "Asia/Kolkata",
  }).format(new Date(iso));
}

function trialDaysLeft(sub: Subscription): number {
  if (!sub.trial_ends_at) return 0;
  return Math.max(
    0,
    Math.ceil((new Date(sub.trial_ends_at).getTime() - CANON_NOW.getTime()) / 86_400_000),
  );
}

/** B4 — the current-plan view. Limits grid and billing details are Acme frame content. */
function CurrentView({ org, sub }: { org: string; sub: Subscription | undefined }) {
  const navigate = useNavigate();
  const methods = usePaymentMethods(org);
  const changePlan = useChangePlan(org);
  const problem = changePlan.error instanceof ProblemError ? changePlan.error.problem : undefined;
  const planName = sub ? planLabel(sub.plan) : "Business";

  return (
    <>
      <Pghead
        title="Billing · Payment & plan"
        sub="How Acme pays, what the plan includes, and exactly what happens when a charge fails"
      />

      <div className="grid grid-cols-2 items-start gap-3.5">
        <Card className="flex flex-col gap-3 p-4">
          <div className="flex items-center gap-2">
            <span className="text-[14px] font-semibold">{planName}</span>
            <Pill tone="st">current</Pill>
            <span className="mono ml-auto text-[12.5px]">
              {PLAN_PRICE[sub?.plan ?? "business"]}
            </span>
          </div>
          <div className="grid grid-cols-2 gap-x-3 gap-y-2.5">
            <div>
              <Eyebrow>Seats</Eyebrow>
              <div className="mono mt-0.5 text-[12.5px]">12 / 20</div>
            </div>
            <div>
              <Eyebrow>Egress incl.</Eyebrow>
              <div className="mono mt-0.5 text-[12.5px]">100 GB</div>
            </div>
            <div>
              <Eyebrow>PITR</Eyebrow>
              <div className="mono mt-0.5 text-[12.5px]">30 d</div>
            </div>
            <div>
              <Eyebrow>Support</Eyebrow>
              <div className="mt-0.5 text-[12.5px]">business hours</div>
            </div>
          </div>
          <p className="text-[11px] text-ink3">
            Enterprise adds dedicated cells & private regions · audit export · 99.95% SLA
          </p>
          <div className="flex flex-wrap items-center gap-2">
            <Btn
              variant="s"
              onClick={() => navigate({ to: "/$org/billing/plans", params: { org } })}
            >
              Compare plans (B5)
            </Btn>
            <Btn
              variant="s"
              onClick={() => {
                window.location.href = "mailto:sales@steloit.com";
              }}
            >
              Talk to us
            </Btn>
            <Btn
              variant="s"
              onClick={() => changePlan.mutate({ path: { org }, body: { plan: "free" } })}
            >
              Downgrade to Free…
            </Btn>
          </div>
          {problem ? (
            <div className="flex items-start gap-2 text-[11.5px]">
              <Pill tone="err">blocked</Pill>
              <span>{problem.detail} — the button says why, like every blocked action here</span>
            </div>
          ) : null}
        </Card>

        <Card className="flex flex-col gap-3 p-4">
          <Eyebrow>Payment methods</Eyebrow>
          {(methods.data ?? []).map((pm) => (
            <div key={pm.id} className="flex items-center gap-2 text-[12.5px]">
              <span className="font-medium">
                {pm.brand} ·· {pm.last4}
              </span>
              {pm.backup ? <Pill tone="mut">backup</Pill> : <Pill tone="st">default</Pill>}
              {pm.expires ? (
                <span className="mono text-[11px] text-ink3">
                  exp {pm.expires.replace("/", " / ")}
                </span>
              ) : null}
              <span className="ml-auto">
                <Btn variant="s" disabled disabledReason="Card management lands in Phase 4">
                  Replace…
                </Btn>
              </span>
            </div>
          ))}
          <div className="flex items-center gap-2 text-[12.5px]">
            <span className="text-ink2">Backup method</span>
            <Pill tone="mut">none</Pill>
            <span className="ml-auto">
              <Btn variant="s" disabled disabledReason="Card management lands in Phase 4">
                Add backup
              </Btn>
            </span>
          </div>
          <p className="text-[11px] text-ink3">
            A backup card is tried automatically before any dunning starts. Cards are stored with
            the processor — Steloit keeps a token, never the number.
          </p>
        </Card>

        <Card className="flex flex-col gap-2.5 p-4">
          <Eyebrow>Billing details</Eyebrow>
          <div className="flex items-baseline justify-between gap-3 text-[12.5px]">
            <span className="text-ink3">Company</span>
            <span>Acme Technologies Pvt Ltd</span>
          </div>
          <div className="flex items-baseline justify-between gap-3 text-[12.5px]">
            <span className="text-ink3">GSTIN</span>
            <span className="mono">29ABCDE1234F1Z5</span>
          </div>
          <div className="flex items-baseline justify-between gap-3 text-[12.5px]">
            <span className="text-ink3">Address</span>
            <span>Indiranagar, Bengaluru 560038, IN</span>
          </div>
          <p className="text-[11px] text-ink3">
            Invoices to invoices@acme.dev + the Owner, always.
          </p>
        </Card>

        <Card className="flex flex-col gap-2.5 p-4">
          <div className="text-[12.5px] font-semibold">
            If a charge fails — the whole policy, up front
          </div>
          <div className="flex items-baseline gap-3 text-[12px]">
            <span className="mono w-16 shrink-0 text-ink3">day 0–7</span>
            <span>
              3 automatic retries (backup card first) · Owner + Admins notified in-app and by email
              (N-series)
            </span>
          </div>
          <div className="flex items-baseline gap-3 text-[12px]">
            <span className="mono w-16 shrink-0 text-ink3">day 7–21</span>
            <span>
              console banner for Admins · new provisioning paused · running services untouched
            </span>
          </div>
          <div className="flex items-baseline gap-3 text-[12px]">
            <span className="mono w-16 shrink-0 text-ink3">day 21+</span>
            <span>
              services suspend (state kept) · no data deleted for billing reasons within 90 days
            </span>
          </div>
          <p className="border-hair border-t pt-2 text-[11px] text-ink3">
            Nothing in this column is a surprise reserved for the incident — it's the contract,
            printed where you set up the card.
          </p>
        </Card>
      </div>
    </>
  );
}

/** B9 — the Borealis Pro trial, reachable via the org switcher. */
function TrialView({ org, sub }: { org: string; sub: Subscription }) {
  const navigate = useNavigate();
  const changePlan = useChangePlan(org);
  const cancelPlan = useCancelPlan(org);
  const days = trialDaysLeft(sub);

  const _choosePro = () =>
    changePlan.mutate(
      { path: { org }, body: { plan: "pro" } },
      {
        onSuccess: () => toast.success("Pro is active — $29/mo starts at the anchor."),
        onError: (err) => toast.error(errorMessage(err)),
      },
    );

  return (
    <>
      {/* The frame's trial banner is steel-toned; Banner's contract is only
          default | warn (16-qa), so default carries it. */}
      <Banner tone="default">
        <span>
          Pro trial · {days} days left. Everything Pro, no card required. This banner is the only
          nag — it appears here and in the org switcher, nowhere else.
        </span>
        <span className="ml-auto shrink-0">
          <Btn
            variant="p"
            onClick={() => navigate({ to: "/$org/billing/confirm", params: { org } })}
          >
            Choose Pro — $29/mo
          </Btn>
        </span>
      </Banner>

      <Pghead
        title={
          <span className="flex items-center gap-2.5">
            Billing · Payment & plan
            <Pill tone="st">trial</Pill>
          </span>
        }
        sub="Borealis · Pro trial — what happens then is written below, not sprung on you."
      />

      <div className="grid grid-cols-2 items-start gap-3.5">
        <Card className="flex flex-col gap-3 p-4">
          <div className="text-[12.5px] font-semibold">
            When the trial ends — the whole contract
          </div>
          <div className="flex items-baseline gap-2.5 text-[12px]">
            <Dot tone="ok" />
            <span>
              If you choose Pro — $29/mo starts at the anchor. Nothing changes in your projects.
            </span>
          </div>
          <div className="flex items-baseline gap-2.5 text-[12px]">
            <Dot tone="warn" />
            <span>
              If you don't — the org moves to Free limits. Your 1 project and services keep running
              and keep billing usage; anything over Free limits (extra previews, retention past 3
              days) winds down with 7 days' notice.
            </span>
          </div>
          <div className="flex items-baseline gap-2.5 text-[12px]">
            <Dot tone="ok" />
            <span>
              Either way — nothing is deleted. No card was taken, so nothing is charged silently.
            </span>
          </div>
          <p className="border-hair border-t pt-2 text-[11px] text-ink3">
            Trial reminders: bell + email at 7 days and 1 day. No countdown modals, no dark patterns
            — the trial ends the way this page says it does.
          </p>
        </Card>

        <Card className="flex flex-col gap-2.5 p-4">
          <Eyebrow>Trial usage so far</Eyebrow>
          <div className="flex items-baseline justify-between gap-3 text-[12.5px]">
            <span className="text-ink3">Resources (usage)</span>
            <span className="mono">$4.12</span>
          </div>
          <div className="flex items-baseline justify-between gap-3 text-[12.5px]">
            <span className="text-ink3">Plan during trial</span>
            <span className="mono">$0</span>
          </div>
          <div className="flex items-baseline justify-between gap-3 text-[12.5px]">
            <span className="text-ink3">Members</span>
            <span className="mono">2 / 5</span>
          </div>
          <p className="text-[11px] text-ink3">
            Usage bills regardless of plan — the trial waives the plan fee, not the meter. Same rule
            as everywhere.
          </p>
        </Card>
      </div>

      <div className="flex items-center gap-2">
        <Btn variant="p" onClick={() => navigate({ to: "/$org/billing/confirm", params: { org } })}>
          Choose Pro — $29/mo
        </Btn>
        <Btn
          variant="s"
          onClick={() =>
            cancelPlan.mutate(
              { path: { org } },
              { onError: (err) => toast.error(errorMessage(err)) },
            )
          }
        >
          Stay on Free at trial end
        </Btn>
      </div>
    </>
  );
}

/** B12 — cancelled at anchor: the wind-down contract, and the way back. */
function CancelledView({ org, sub }: { org: string; sub: Subscription }) {
  const changePlan = useChangePlan(org);
  const anchor = sub.wind_down?.plan_ends_at
    ? fmtAnchorDate(sub.wind_down.plan_ends_at)
    : "the anchor";

  return (
    <>
      <Pghead
        title={
          <span className="flex items-center gap-2.5">
            Billing · Payment & plan
            <Pill tone="mut">cancelled at anchor</Pill>
          </span>
        }
      />

      <div className="grid grid-cols-2 items-start gap-3.5">
        <Card className="flex flex-col gap-3 p-4">
          <div className="text-[12.5px] font-semibold">On {anchor}, Borealis moves to Free</div>
          <div className="flex items-baseline gap-2.5 text-[12px]">
            <Dot tone="ok" />
            <span>
              Keeps running: your project, services, data, domains — and usage keeps billing at the
              same unit prices. Cancelling the plan is not deleting resources.
            </span>
          </div>
          <div className="flex items-baseline gap-2.5 text-[12px]">
            <Dot tone="warn" />
            <span>
              Winds down: preview environments (Free has none) close when their PRs do; log
              retention shortens to 3 days going forward — nothing already recorded is retro-deleted
              early.
            </span>
          </div>
          <div className="flex items-baseline gap-2.5 text-[12px]">
            <Dot tone="ok" />
            <span>
              Nothing is deleted. Not at cancellation, not after. Deletion is only ever yours to do
              — or the day-90 billing rule with final notice.
            </span>
          </div>
        </Card>

        <Card className="flex flex-col gap-3 p-4">
          <div className="text-[12.5px] font-semibold">Coming back</div>
          <p className="text-[12px]">
            Reactivation is one click, any time — nothing was deleted in between.
          </p>
          <div>
            <Btn
              variant="p"
              onClick={() =>
                changePlan.mutate(
                  { path: { org }, body: { plan: "pro" } },
                  {
                    onSuccess: () => toast.success("Pro is active — $29/mo starts at the anchor."),
                    onError: (err) => toast.error(errorMessage(err)),
                  },
                )
              }
            >
              Resume Pro (undo cancellation)
            </Btn>
          </div>
        </Card>
      </div>
    </>
  );
}

function PaymentPage() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const { state } = Route.useSearch();
  const sub = useSubscription(org, state);

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project="ecommerce"
        plan={
          sub.data ? planLabel(sub.data.plan) : orgRecord ? planLabel(orgRecord.plan) : "Business"
        }
        active="b-payment"
      />
      <main className="main">
        <div className="pgpad">
          {sub.data?.status === "grace" ||
          sub.data?.status === "provisioning_paused" ||
          sub.data?.dunning?.state === "provisioning_paused" ? (
            <DunningView org={org} sub={sub.data} />
          ) : sub.data?.status === "trial" ? (
            <TrialView org={org} sub={sub.data} />
          ) : sub.data?.status === "cancelled_at_anchor" ? (
            <CancelledView org={org} sub={sub.data} />
          ) : (
            <CurrentView org={org} sub={sub.data} />
          )}
        </div>
      </main>
    </>
  );
}

/**
 * B10 · Failed payment — the dunning contract from Payment & plan, now
 * lived, with the exact position marked. Reachable in canon mode via the
 * ?state=dunning_day8 demo param (the canon subscription never enters grace
 * on its own — finding). Frame Apr dates reconciled to the fixtures' cycle.
 */
function DunningView({ sub }: { org?: string; sub: Subscription }) {
  const day = sub.dunning?.day ?? 8;
  return (
    <>
      <Banner tone="warn">
        <span className="flex-1">
          <b>Payment failed {day} days ago.</b> Grace continues — services run normally; new
          provisioning is paused (day 7–21 of the policy). Next retry tomorrow.
        </span>
        <Btn
          variant="s"
          className="h-6 px-2 text-[10.5px]"
          disabled
          disabledReason="Card updates land with the payment processor"
        >
          Update card
        </Btn>
        <Btn
          variant="gh"
          className="h-6 px-2 text-[10.5px]"
          disabled
          disabledReason="Retries run on the printed schedule"
        >
          Retry now
        </Btn>
      </Banner>
      <Pghead
        title={
          <span className="flex items-center gap-2.5">
            Billing · Overview <Pill tone="err">payment failed</Pill>
          </span>
        }
        sub="Borealis · the dunning contract from Payment & plan, now lived — with your exact position marked. No step here is a surprise."
      />
      <div className="flex gap-3.5">
        <Card className="flex flex-[1.3] flex-col gap-3 p-4">
          <Eyebrow>Where you are in the policy</Eyebrow>
          <div className="flex items-start gap-3 text-[12px]">
            <span className="text-ok">✓</span>
            <span>
              <b>Day 0 — charge failed</b>
              <span className="mono block text-[10.5px] text-ink3">
                Visa ··8841 declined · Owner + Admins notified (bell + email)
              </span>
            </span>
          </div>
          <div className="flex items-start gap-3 text-[12px]">
            <span className="text-ok">✓</span>
            <span>
              <b>Day 0–7 — automatic retries</b>
              <span className="mono block text-[10.5px] text-ink3">
                3 retries · backup card first (none on file) · all declined
              </span>
            </span>
          </div>
          <div className="flex items-start gap-3 rounded-lg border border-warn/40 bg-warn-tint/40 p-2.5 text-[12px]">
            <span className="dot warn mt-1" />
            <span>
              <b>Day 7–21 — you are here (day {day})</b>
              <span className="block text-[10.5px] text-ink3">
                Console banner for Admins · new provisioning paused · running services untouched ·
                retries continue
              </span>
            </span>
          </div>
          <div className="flex items-start gap-3 text-[12px] text-ink3">
            <span>4</span>
            <span>
              <b className="text-ink2">Day 21+ — suspension</b>
              <span className="block text-[10.5px]">
                Services suspend with state kept · resume instantly on payment
              </span>
            </span>
          </div>
          <div className="flex items-start gap-3 text-[12px] text-ink3">
            <span>5</span>
            <span>
              <b className="text-ink2">Day 90</b>
              <span className="block text-[10.5px]">
                Only now can data be deleted for billing reasons — with 14 days' final notice
              </span>
            </span>
          </div>
        </Card>
        <div className="flex flex-1 flex-col gap-3">
          <Card className="flex flex-col gap-2.5 p-4">
            <Eyebrow>Fix it</Eyebrow>
            <div className="flex justify-between text-[12px]">
              <span className="text-ink3">Outstanding</span>
              <span className="mono">$29.00 + $4.12 usage</span>
            </div>
            <div className="flex justify-between text-[12px]">
              <span className="text-ink3">Card on file</span>
              <span className="mono">Visa ··8841 declined</span>
            </div>
            <Btn variant="p" disabled disabledReason="Card updates land with the payment processor">
              Update card & pay now
            </Btn>
            <Btn variant="s" disabled disabledReason="Backup cards land with the payment processor">
              Add a backup card
            </Btn>
            <p className="text-[10.5px] text-ink3">
              Payment clears everything instantly — banner gone, provisioning unpaused, no residue.
            </p>
          </Card>
          <Card className="p-3.5 text-[11.5px] leading-relaxed text-ink3">
            Grace period is a promise, not a favor: it's the same schedule printed on Payment & plan
            (B4) since day one.
          </Card>
        </div>
      </div>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/billing/payment")({
  validateSearch: (search: Record<string, unknown>): { state?: string } => ({
    state: typeof search.state === "string" ? search.state : undefined,
  }),
  component: PaymentPage,
});
