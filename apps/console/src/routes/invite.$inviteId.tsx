import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Icon, type IconId } from "@/design-system/icon";
import { Pill } from "@/design-system/pill";
import { matchesHint } from "@/features/invites/hint";
import {
  useAcceptInvite,
  useDeclineInvite,
  useInvitePublic,
  useRenewInvite,
} from "@/features/invites/hooks";
import { ProblemError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { cn } from "@/lib/utils";

/**
 * A6 · Invitation accept/decline + A7 · failure states. Every failure state
 * names its cause where security allows, and every one has a working way
 * forward — no dead ends (A7 is the reference).
 */

function CenterCard({
  icon,
  tone,
  children,
}: {
  icon?: IconId;
  tone?: "warn" | "err";
  children: React.ReactNode;
}) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-canvas p-6">
      <Card className="flex w-[460px] flex-col gap-4 p-6">
        {icon ? (
          <Icon
            id={icon}
            className={cn(
              "h-6 w-6",
              tone === "warn" ? "text-warn" : tone === "err" ? "text-err" : "text-ink2",
            )}
          />
        ) : null}
        {children}
      </Card>
    </div>
  );
}

function InvitePage() {
  const { inviteId } = Route.useParams();
  const navigate = useNavigate();
  const session = useSession();
  const invite = useInvitePublic(inviteId);
  const accept = useAcceptInvite();
  const decline = useDeclineInvite();
  const renew = useRenewInvite();
  const [declined, setDeclined] = useState(false);
  const [renewed, setRenewed] = useState(false);

  if (invite.isPending) {
    return (
      <CenterCard>
        <div className="h-4 w-2/3 animate-pulse rounded bg-surface2" />
        <div className="h-3 w-1/2 animate-pulse rounded bg-surface2" />
      </CenterCard>
    );
  }

  // A7 — failure states
  if (invite.isError) {
    const status = invite.error instanceof ProblemError ? invite.error.problem.status : 500;
    if (status === 410) {
      return (
        <CenterCard icon="s-cron" tone="warn">
          <h1 className="h1">This invitation has expired</h1>
          <p className="text-12p5 leading-relaxed text-ink2">
            Invitations are valid for <b>7 days</b> — this one expired 2 days ago. They expire so
            nobody joins on a stale link; membership stays auditable. <b>One click fixes it:</b>{" "}
            Priya gets a bell and an email, and the new link arrives at <b>marco@acme.dev</b>.
          </p>
          {renewed ? (
            <p className="text-12p5 text-ok">Requested — the inviter has been notified.</p>
          ) : (
            <div className="flex items-center gap-3">
              <Btn
                variant="p"
                onClick={() => renew.mutate(inviteId, { onSuccess: () => setRenewed(true) })}
              >
                Request a new invitation
              </Btn>
              <Link to="/login" className="text-12 font-medium text-steel">
                Go to sign in
              </Link>
            </div>
          )}
          <div className="mono text-10p5 text-ink3">inv_7d21aa · expired 04 Jul 2026</div>
        </CenterCard>
      );
    }
    return (
      <CenterCard icon="s-shield">
        <h1 className="h1">This link isn't valid</h1>
        <p className="text-12p5 leading-relaxed text-ink2">
          <b>Most common reason: it was already used.</b> If you accepted earlier — even on another
          device — you're a member; just sign in. Otherwise it was revoked by an admin or truncated
          when copied. For security we can't say which — revocations are recorded in the org audit
          log.
        </p>
        <div className="flex items-center gap-3">
          <Btn variant="p" onClick={() => navigate({ to: "/login" })}>
            Sign in — I may already be a member
          </Btn>
        </div>
        <div className="text-12 text-ink3">Ask Priya for a new link · Contact support</div>
        <div className="mono text-10p5 text-ink3">
          link presented 06 Jul 2026 · nothing was changed by opening it
        </div>
      </CenterCard>
    );
  }

  const data = invite.data;
  const hint = data.email_hint ?? "";
  const roleName = (data.role ?? "developer").replace(/^./, (c) => c.toUpperCase());

  // A7 — wrong account
  if (session && hint && !matchesHint(session.email, hint)) {
    return (
      <CenterCard icon="s-user" tone="err">
        <h1 className="h1">Sent to a different email</h1>
        <p className="text-12p5 leading-relaxed text-ink2">
          This invitation is for <b>{hint}</b>, but you're signed in as <b>{session.email}</b>.
          Invitations are bound to the address they were sent to — accepting from another account
          would put the wrong person in the org. Switch to that account, or ask Priya to invite this
          one instead.
        </p>
        <div className="flex items-center gap-3">
          <Btn variant="s" onClick={() => navigate({ to: "/login" })}>
            Switch account
          </Btn>
        </div>
        <div className="text-12 text-ink3">Ask Priya to invite {session.email}</div>
        <div className="mono text-10p5 text-ink3">
          inv_9d47c2 · unchanged — switching accounts is safe
        </div>
      </CenterCard>
    );
  }

  if (declined) {
    return (
      <CenterCard>
        <h1 className="h1">Invitation declined</h1>
        <p className="text-12p5 leading-relaxed text-ink2">
          {data.inviter_name} is notified and the link becomes invalid. No silent limbo.
        </p>
        <Link to="/login" className="text-12 font-medium text-steel">
          Go to sign in
        </Link>
      </CenterCard>
    );
  }

  // A6 — accept / decline
  return (
    <CenterCard>
      <h1 className="h1 flex items-center gap-2">
        Join <b>{data.org_name}</b> as <Pill tone="st">{roleName}</Pill>
      </h1>
      <div className="text-12 text-ink3">
        Invited by <b className="text-ink1">{data.inviter_name}</b> · Admin · 3 days ago
      </div>
      <Card className="bg-surface2 p-3.5">
        <div className="eyebrow mb-2">What {roleName} means here</div>
        {(data.role_consequences ?? []).map((line) => (
          <p key={line} className="text-12 leading-relaxed text-ink2">
            {line}
          </p>
        ))}
      </Card>
      <ul className="flex flex-col gap-1.5 text-12 text-ink2">
        <li className="flex items-center gap-1.5">
          <Icon id="s-check" className="h-[11px] w-[11px]" /> Accepting gives you access immediately
          — no approval step follows
        </li>
        <li className="flex items-center gap-1.5">
          <Icon id="s-check" className="h-[11px] w-[11px]" /> Joining is logged in the org audit
          trail, like every membership change
        </li>
      </ul>
      {session ? (
        <div className="text-12 text-ink3">
          Signed in as <b className="text-ink1">{session.email}</b> — this invitation was sent to
          you · <span className="text-steel">Not you? Switch account</span>
        </div>
      ) : null}
      <div className="flex items-center gap-3">
        <Btn
          variant="p"
          disabled={accept.isPending}
          onClick={() =>
            accept.mutate(
              { path: { invite: inviteId } },
              {
                onSuccess: () => {
                  toast.success(`Welcome to ${data.org_name}`);
                  navigate({ to: "/" });
                },
              },
            )
          }
        >
          Accept — join {data.org_name}
        </Btn>
        <button
          type="button"
          className="text-12 font-medium text-ink2 hover:text-ink1"
          onClick={() =>
            decline.mutate({ path: { invite: inviteId } }, { onSuccess: () => setDeclined(true) })
          }
        >
          Decline
        </button>
      </div>
      <div className="text-11 text-ink3">
        {data.inviter_name} is notified and the link becomes invalid. No silent limbo.
      </div>
      <div className="mono text-10p5 text-ink3">
        inv_9d47c2 · expires in 6 days · sent to {session?.email ?? hint}
      </div>
    </CenterCard>
  );
}

export const Route = createFileRoute("/invite/$inviteId")({
  component: InvitePage,
});
