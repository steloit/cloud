import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Pill } from "@/design-system/pill";
import { requireSession } from "@/features/auth/guards";
import { OnboardingCard, OnboardingShell } from "@/features/onboarding/onboarding-shell";
import { markWelcomePending } from "@/features/onboarding/welcome-widget";

/** A9 · Onboarding step 4 — connect. All optional — nothing here gates anything. */
function OnboardingConnectPage() {
  const navigate = useNavigate();
  const { org, project } = Route.useSearch();

  return (
    <OnboardingShell step={3}>
      <OnboardingCard
        title="Connect your tools"
        sub="All optional — nothing here gates anything. The console alone is enough."
      >
        <div className="grid grid-cols-1 gap-2.5">
          <Card className="flex flex-col gap-2 p-3.5">
            <div className="flex items-center gap-2">
              <Eyebrow>CLI</Eyebrow>
              <Pill tone="ok">connected</Pill>
            </div>
            <div className="flex flex-wrap gap-2">
              <Copybit>npm i -g steloit</Copybit>
              <Copybit>steloit login</Copybit>
            </div>
            <div className="mono text-[10.5px] text-ok">✓ laptop-cli connected just now</div>
            <div className="text-[10.5px] text-ink3">device token · manage under your profile</div>
          </Card>
          <Card className="flex flex-col gap-2 p-3.5">
            <Eyebrow>Git</Eyebrow>
            <p className="text-[12px] leading-relaxed text-ink2">
              Deploy on push, previews per PR. One org-level app — you pick the repos, no per-repo
              tokens.
            </p>
            <div>
              <Btn variant="s">Connect GitHub</Btn>
            </div>
          </Card>
          <Card className="flex flex-col gap-2 p-3.5">
            <Eyebrow>Just the console</Eyebrow>
            <p className="text-[12px] leading-relaxed text-ink2">
              Everything — creating, observing, deploying — works in the browser. Connect tools
              whenever they earn it.
            </p>
            <div className="mono text-[10.5px] text-ink3">⌘K gets you anywhere</div>
          </Card>
        </div>
        <Btn
          variant="p"
          className="w-full justify-center"
          onClick={() => {
            markWelcomePending();
            navigate({ to: "/$org", params: { org } });
          }}
        >
          Open {org}/{project} →
        </Btn>
      </OnboardingCard>
    </OnboardingShell>
  );
}

export const Route = createFileRoute("/onboarding/connect")({
  beforeLoad: ({ location }) => requireSession(location.href),
  validateSearch: (search: Record<string, unknown>): { org: string; project: string } => ({
    org: typeof search.org === "string" ? search.org : "acme",
    project: typeof search.project === "string" ? search.project : "my-app",
  }),
  component: OnboardingConnectPage,
});
