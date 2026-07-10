import { useMutation } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Copybit } from "@/design-system/copybit";
import { Flabel, Inp } from "@/design-system/inp";
import { requireSession } from "@/features/auth/guards";
import { OnboardingCard, OnboardingShell } from "@/features/onboarding/onboarding-shell";
import { createOrgMutation, errorMessage, type Org } from "@/lib/api";

/** A5 · Onboarding step 1 — organization: identity only, name + home region. */
function OnboardingOrgPage() {
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [error, setError] = useState<string>();
  const create = useMutation(createOrgMutation());

  const slug = name
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, "-")
    .replace(/^-+|-+$/g, "");

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (name.trim().length < 3 || name.trim().length > 32) {
      setError("Organization names are 3–32 characters.");
      return;
    }
    create.mutate(
      { body: { name: name.trim(), home_region: "aws/ap-south-1" } },
      {
        onSuccess: (org: Org) => {
          navigate({ to: "/onboarding/team", search: { org: org.slug } });
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    <OnboardingShell step={0}>
      <OnboardingCard
        title="Create your organization"
        sub="The boundary of identity, billing and policy — projects live inside it."
      >
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <div>
            <Flabel htmlFor="org-name">Organization name</Flabel>
            <Inp
              id="org-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              error={error}
              autoFocus
            />
            <div className="mono mt-1.5 text-[10.5px] text-ink3">
              steloit.app/{slug || "…"} · slug is permanent
            </div>
          </div>
          <div>
            <Flabel htmlFor="region">Home region</Flabel>
            <div className="inp" id="region">
              aws · ap-south-1 · Mumbai
            </div>
            <div className="mt-1.5 text-[10.5px] text-ink3">
              Default for new environments — overridable per environment, and later per cell.
            </div>
          </div>
          <Btn
            variant="p"
            type="submit"
            className="w-full justify-center"
            disabled={create.isPending}
          >
            Continue → Invite your team
          </Btn>
        </form>
        <div className="flex items-center gap-2 text-[11px] text-ink3">
          Prefer the terminal?
          <Copybit>{`steloit org create ${slug || "acme"} --region aws/ap-south-1`}</Copybit>
        </div>
      </OnboardingCard>
    </OnboardingShell>
  );
}

export const Route = createFileRoute("/onboarding/org")({
  beforeLoad: ({ location }) => requireSession(location.href),
  component: OnboardingOrgPage,
});
