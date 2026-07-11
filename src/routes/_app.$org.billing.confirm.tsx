import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { useChangePlan, useSubscription } from "@/features/billing/hooks";
import { useOrgs } from "@/features/org/hooks";
import { errorMessage } from "@/lib/api";

/**
 * B11 · Plan change confirm — review is the whole screen; the button does
 * exactly this and nothing else. Route is a minimal addition under
 * /:org/billing (02-ia lists /billing/plans; the confirm page needed its own
 * URL — finding). Frame's "the 5th" anchor reconciled to the fixtures'
 * anchor_day 1.
 */
function ConfirmPlanPage() {
  const { org } = Route.useParams();
  const navigate = useNavigate();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const sub = useSubscription(org);
  const changePlan = useChangePlan(org);
  const anchor = sub.data?.anchor_day ?? 1;
  const anchorLabel = `the ${anchor}${anchor === 1 ? "st" : anchor === 2 ? "nd" : anchor === 3 ? "rd" : "th"}, monthly`;

  const confirm = () =>
    changePlan.mutate(
      { path: { org }, body: { plan: "pro" } },
      {
        onSuccess: () => {
          toast.success("Pro started — $29.00 charged");
          navigate({ to: "/$org/billing/payment", params: { org } });
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project="ecommerce"
        plan="Pro trial"
        active="b-payment"
      />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Billing · Confirm plan — Pro, $29/mo"
            sub="Borealis · converting from trial · review is the whole screen; the button does exactly this and nothing else."
          />
          <div className="flex gap-3.5">
            <Card className="flex max-w-[560px] flex-1 flex-col gap-2.5 p-4">
              {[
                ["Plan", "Pro · monthly"],
                ["Starts", "today — trial fee never billed"],
                ["Billing anchor", anchorLabel],
                ["First charge · today", "$29.00"],
                ["Usage to date (bills at the anchor)", "$4.12"],
                ["Card", "Visa ··8841"],
              ].map(([k, v]) => (
                <div key={k} className="fieldrow">
                  <span className="k">{k}</span>
                  <span className="mono text-[12px]">{v}</span>
                </div>
              ))}
              <p className="border-hair border-t pt-2.5 text-[11.5px] leading-relaxed text-ink2">
                Mid-cycle changes, stated once: upgrades apply immediately and the difference is
                prorated to your anchor; downgrades apply at the next anchor so you keep what you
                paid for. Seats added mid-cycle prorate the same way.
              </p>
              <div className="flex gap-2.5">
                <Btn variant="p" onClick={confirm} disabled={changePlan.isPending}>
                  Start Pro — charge $29.00 now
                </Btn>
                <Btn
                  variant="s"
                  onClick={() => navigate({ to: "/$org/billing/payment", params: { org } })}
                >
                  Back
                </Btn>
              </div>
            </Card>
            <div className="flex w-[320px] shrink-0 flex-col gap-3">
              <Card className="flex flex-col gap-1.5 p-3.5">
                <b className="text-[12.5px]">What changes right now</b>
                <p className="text-[11.5px] leading-relaxed text-ink2">
                  Nothing in your projects — you already had Pro capabilities on trial. The only
                  change is the charge and the anchor date.
                </p>
              </Card>
              <Card className="p-3.5 text-[11.5px] leading-relaxed text-ink3">
                Cancel anytime — the plan runs to the anchor you paid through, then drops to Free by
                the written rules (B12). No refunds needed because nothing bills ahead of use except
                the flat fee.
              </Card>
            </div>
          </div>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/billing/confirm")({
  component: ConfirmPlanPage,
});
