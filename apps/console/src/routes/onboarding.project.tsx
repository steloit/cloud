import { useMutation } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Copybit } from "@/design-system/copybit";
import { Flabel, Inp } from "@/design-system/inp";
import { requireSession } from "@/features/auth/guards";
import { OnboardingCard, OnboardingShell } from "@/features/onboarding/onboarding-shell";
import { createProjectMutation, errorMessage } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * A8 · Onboarding step 3 — first project. Blank is the highlighted default;
 * templates carry price before anything exists.
 */

const STARTERS = [
  {
    key: "blank",
    name: "Blank",
    desc: "Empty project · add services when you need them",
    price: "$0/mo",
  },
  {
    key: "saas-starter",
    name: "saas-starter",
    desc: "Web + Postgres + cache + queue",
    price: "$96/mo · shown before create",
  },
  { key: "docs-site", name: "docs-site", desc: "Static web + storage", price: "$14/mo" },
];

function OnboardingProjectPage() {
  const navigate = useNavigate();
  const { org } = Route.useSearch();
  const [name, setName] = useState("my-app");
  const [starter, setStarter] = useState("blank");
  const create = useMutation(createProjectMutation());

  const submit = () => {
    create.mutate(
      { path: { org }, body: { name } },
      {
        onSuccess: () => navigate({ to: "/onboarding/connect", search: { org, project: name } }),
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    <OnboardingShell step={2}>
      <OnboardingCard
        title="Create your first project"
        sub="Projects are free — services are priced, and every price is shown before anything exists."
      >
        <div>
          <Flabel htmlFor="project-name">Project name</Flabel>
          <Inp id="project-name" value={name} onChange={(e) => setName(e.target.value)} autoFocus />
          <div className="mono mt-1.5 text-10p5 text-ink3">
            {org}/{name || "…"} · environments default to aws · ap-south-1 (from Acme) — change
            anytime
          </div>
        </div>

        <div>
          <Flabel>Start from</Flabel>
          <div className="grid grid-cols-3 gap-2.5">
            {STARTERS.map((s) => (
              <button
                key={s.key}
                type="button"
                onClick={() => setStarter(s.key)}
                className={cn(
                  "card flex flex-col gap-1.5 p-3 text-left",
                  starter === s.key && "border-steel",
                )}
              >
                <span className="text-12p5 font-semibold">{s.name}</span>
                <span className="text-10p5 leading-snug text-ink3">{s.desc}</span>
                <span className="mono text-10p5 text-ink2">{s.price}</span>
              </button>
            ))}
          </div>
          <div className="mt-1.5 text-10p5 text-ink3">
            All templates → in the console; each opens a full estimate before anything is
            provisioned.
          </div>
        </div>

        <Btn
          variant="p"
          className="w-full justify-center"
          onClick={submit}
          disabled={create.isPending}
        >
          Create {name || "my-app"} → Connect
        </Btn>
        <div className="text-center">
          <Link
            to="/onboarding/connect"
            search={{ org, project: name }}
            className="text-11p5 font-medium text-ink3 hover:text-ink1"
          >
            Skip for now — straight to the console
          </Link>
        </div>
        <div className="flex justify-center">
          <Copybit>{`steloit project create ${name || "my-app"}`}</Copybit>
        </div>
      </OnboardingCard>
    </OnboardingShell>
  );
}

export const Route = createFileRoute("/onboarding/project")({
  beforeLoad: ({ location }) => requireSession(location.href),
  validateSearch: (search: Record<string, unknown>): { org: string } => ({
    org: typeof search.org === "string" ? search.org : "acme",
  }),
  component: OnboardingProjectPage,
});
