import type { ReactNode } from "react";
import { Icon } from "@/design-system/icon";
import { Steps } from "@/design-system/steps";

export const ONBOARDING_STEPS = ["Organization", "Team", "First project", "Connect"];

/** Centered single-column onboarding layout with the 4-step progress bar. */
export function OnboardingShell({ step, children }: { step: number; children: ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col items-center gap-8 bg-canvas px-6 py-12">
      <div className="flex items-center gap-2.5">
        <Icon id="s-hex" className="h-[22px] w-[22px] text-steel" />
        <span className="text-[14px] font-semibold">Steloit</span>
      </div>
      <Steps labels={ONBOARDING_STEPS} current={step} />
      <div className="w-full max-w-[520px]">{children}</div>
    </div>
  );
}

export function OnboardingCard({
  title,
  sub,
  children,
}: {
  title: string;
  sub: string;
  children: ReactNode;
}) {
  return (
    <div className="card flex flex-col gap-4 p-6">
      <div>
        <h1 className="h1">{title}</h1>
        <div className="hsub">{sub}</div>
      </div>
      {children}
    </div>
  );
}
