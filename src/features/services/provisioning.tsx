import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Pill } from "@/design-system/pill";
import type { Service } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * C4 · Provisioning — the honest in-between. Step timeline from
 * Service.provisioning_steps (allocate → configure → network/credentials →
 * first backup → ready); metering begins at ready, never at click.
 */

const STEP_NOTES: Record<string, string> = {
  "Allocate compute": "4s",
  "Private network & scoped credentials": "in progress",
};

export function ProvisioningView({ svc, env }: { svc: Service; env: string }) {
  const steps = svc.provisioning_steps ?? [];

  return (
    <>
      <Card className="flex flex-col gap-3 p-4">
        {steps.map((step) => {
          const status = step.status ?? "pending";
          return (
            <div key={step.step} className="flex items-center gap-3 text-[12.5px]">
              {status === "done" ? (
                <span className="text-ok">✓</span>
              ) : status === "active" ? (
                <span className="dot prov" />
              ) : (
                <span className="dot sus" />
              )}
              <span
                className={cn(
                  status === "active" && "font-semibold",
                  status === "pending" && "text-ink3",
                )}
              >
                {step.step}
              </span>
              <span className="mono ml-auto text-[10.5px] text-ink3">
                {STEP_NOTES[step.step ?? ""] ?? (status === "done" ? "✓" : "")}
              </span>
            </div>
          );
        })}
      </Card>
      <div className="grid grid-cols-2 gap-3.5">
        <Card className="flex flex-col gap-2 p-4">
          <Eyebrow>While you wait</Eyebrow>
          <p className="text-[12px] text-ink2">
            Your connect command is ready — it will work the moment status flips:
          </p>
          <Copybit>{`steloit db connect ${svc.name} --env ${env}`}</Copybit>
        </Card>
        <Card className="flex flex-col gap-2 p-4">
          <Eyebrow>Billing</Eyebrow>
          <p className="text-[12px] leading-relaxed text-ink2">
            <b>Nothing has billed yet.</b> Metering begins at <Pill tone="ok">ready</Pill>. If
            provisioning fails, nothing ever bills — and the failure lands in the audit log with a
            reason.
          </p>
        </Card>
      </div>
    </>
  );
}
