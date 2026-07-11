import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { Mark } from "@/app/shell/mark";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Glyph } from "@/design-system/glyph";
import { Icon } from "@/design-system/icon";
import { Dot } from "@/design-system/pill";
import { useOrgs } from "@/features/org/hooks";
import { initialsOf, useSession, useSessionStore } from "@/lib/session";

/**
 * E4 · Welcome — authed but in no organization. The orchestrator redirects
 * here when the org list comes back empty. No rail: there's no org to scope it.
 */
function WelcomePage() {
  const session = useSession();
  const signOut = useSessionStore((s) => s.signOut);
  const navigate = useNavigate();
  const orgs = useOrgs();
  const email = session?.email ?? "you@example.dev";

  return (
    <div className="flex h-screen flex-col bg-canvas">
      <header className="ctx">
        <Mark />
        <span className="text-13 font-semibold">Steloit</span>
        <span className="flex-1" />
        <a
          className="icb"
          href="https://docs.steloit.app"
          target="_blank"
          rel="noreferrer"
          aria-label="Documentation"
        >
          <Icon id="s-doc" />
        </a>
        <span className="uav" title={session?.name}>
          {session ? initialsOf(session.name) : "?"}
        </span>
      </header>

      <div className="flex flex-1 flex-col items-center justify-center gap-5 overflow-y-auto px-6 py-10">
        <div className="text-center">
          <h1 className="h1">You're not in any organization yet</h1>
          <p className="hsub mx-auto max-w-[460px]">
            Everything on Steloit — projects, billing, teammates — lives inside an organization.
            Create your own, or join one you've been invited to.
          </p>
        </div>

        <div className="flex gap-3.5">
          <Card className="flex w-[300px] flex-col gap-3 border-steel p-5">
            <Glyph id="s-org" />
            <div className="text-13 font-semibold">Create an organization</div>
            <p className="text-11p5 leading-relaxed text-ink3">
              You'll own billing and policies, and can invite your team. Free tier included —
              nothing bills until you provision.
            </p>
            <Link to="/onboarding/org">
              <Btn variant="p">Create organization</Btn>
            </Link>
            <span className="mono text-10p5 text-ink3">steloit org create</span>
          </Card>

          <Card className="flex w-[300px] flex-col gap-3 p-5">
            <Glyph id="s-users" />
            <div className="text-13 font-semibold">Join an organization</div>
            <p className="text-11p5 leading-relaxed text-ink3">
              Ask an admin to invite <span className="mono">{email}</span>. Invitations appear here
              the moment they're sent — no refresh needed.
            </p>
            <Btn variant="s">I have an invite link</Btn>
            <span className="text-10p5 text-ink3">or open the link from your email</span>
          </Card>
        </div>

        <Card className="flex w-[622px] items-center gap-3 p-4">
          <Dot tone="sus" />
          <span className="flex-1 text-11p5 text-ink3">
            No pending invitations for <span className="mono">{email}</span> — we're watching for
            them automatically.
          </span>
          <Btn variant="s" onClick={() => orgs.refetch()}>
            Check again
          </Btn>
        </Card>

        <div className="flex items-center gap-2 text-11 text-ink3">
          Signed in as <span className="mono">{email}</span> ·
          <button
            type="button"
            className="font-medium text-steel"
            onClick={() => {
              signOut();
              navigate({ to: "/login" });
            }}
          >
            Sign out
          </button>
        </div>
      </div>
    </div>
  );
}

export const Route = createFileRoute("/_app/welcome")({
  component: WelcomePage,
});
