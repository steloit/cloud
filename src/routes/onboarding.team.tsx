import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Copybit } from "@/design-system/copybit";
import { Icon } from "@/design-system/icon";
import { Flabel, Inp } from "@/design-system/inp";
import { requireSession } from "@/features/auth/guards";
import { useCreateInvite } from "@/features/invites/hooks";
import { OnboardingCard, OnboardingShell } from "@/features/onboarding/onboarding-shell";
import { errorMessage } from "@/lib/api";

/** A11 · Onboarding step 2 — invite your team. First-class Skip. */

interface PendingInvite {
  email: string;
  role: "Admin" | "Developer" | "Billing";
}

function OnboardingTeamPage() {
  const navigate = useNavigate();
  const { org } = Route.useSearch();
  const createInvite = useCreateInvite();
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<PendingInvite["role"]>("Developer");
  const [invites, setInvites] = useState<PendingInvite[]>([]);

  const add = () => {
    // Paste multiple emails separated by commas — roles default to Developer.
    const parts = email
      .split(",")
      .map((p) => p.trim())
      .filter((p) => p.includes("@"));
    if (parts.length === 0) return;
    setInvites((prev) => [
      ...prev,
      ...parts.map((p, i) => ({
        email: p,
        role: parts.length > 1 && i > 0 ? ("Developer" as const) : role,
      })),
    ]);
    setEmail("");
  };

  const next = () => {
    for (const invite of invites) {
      createInvite.mutate(
        {
          path: { org },
          body: {
            email: invite.email,
            role: invite.role.toLowerCase() as "admin" | "developer" | "billing",
          },
        },
        { onError: (err) => toast.error(errorMessage(err)) },
      );
    }
    navigate({ to: "/onboarding/project", search: { org } });
  };

  return (
    <OnboardingShell step={1}>
      <OnboardingCard
        title="Invite your team"
        sub="Members join the organization and get a role — change roles or add people anytime under Settings."
      >
        <div>
          <Flabel htmlFor="invite-email">Invite by email</Flabel>
          <div className="flex gap-2">
            <Inp
              id="invite-email"
              type="text"
              placeholder="name@company.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  add();
                }
              }}
              className="flex-1"
            />
            <select
              className="inp w-[130px]"
              value={role}
              onChange={(e) => setRole(e.target.value as PendingInvite["role"])}
              aria-label="Role"
            >
              <option>Admin</option>
              <option>Developer</option>
              <option>Billing</option>
            </select>
            <Btn variant="s" onClick={add}>
              Add
            </Btn>
          </div>
          <div className="mt-1.5 text-[10.5px] text-ink3">
            or paste multiple emails separated by commas — roles default to Developer
          </div>
        </div>

        {invites.length > 0 ? (
          <div>
            <div className="flabel">
              <span>Pending invites</span>
              <span className="font-normal text-ink3">
                {invites.length} invited · 3 seats left on Pro
              </span>
            </div>
            <div className="flex flex-col gap-1.5">
              {invites.map((invite, idx) => (
                <div
                  key={invite.email}
                  className="flex items-center gap-2.5 rounded-lg border border-hair px-3 py-2"
                >
                  <span className="uav">{invite.email.slice(0, 1).toUpperCase()}</span>
                  <span className="mono flex-1 text-[11.5px]">{invite.email}</span>
                  <span className="text-[11.5px] text-ink2">{invite.role} ▾</span>
                  <button
                    type="button"
                    className="icb h-6 w-6"
                    aria-label={`Remove ${invite.email}`}
                    onClick={() => setInvites((prev) => prev.filter((_, i) => i !== idx))}
                  >
                    <Icon id="s-x" className="h-3 w-3" />
                  </button>
                </div>
              ))}
            </div>
          </div>
        ) : null}

        <Btn variant="p" className="w-full justify-center" onClick={next}>
          Continue → First project
        </Btn>
        <div className="text-center">
          <Link
            to="/onboarding/project"
            search={{ org }}
            className="text-[11.5px] font-medium text-ink3 hover:text-ink1"
          >
            Skip — I'll invite people later
          </Link>
        </div>
        <div className="flex justify-center">
          <Copybit>steloit members invite marco@acme.dev --role developer</Copybit>
        </div>
      </OnboardingCard>
    </OnboardingShell>
  );
}

export const Route = createFileRoute("/onboarding/team")({
  beforeLoad: ({ location }) => requireSession(location.href),
  validateSearch: (search: Record<string, unknown>): { org: string } => ({
    org: typeof search.org === "string" ? search.org : "acme",
  }),
  component: OnboardingTeamPage,
});
