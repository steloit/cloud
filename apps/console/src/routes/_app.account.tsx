import { createFileRoute, Outlet, useLocation } from "@tanstack/react-router";
import { Mark } from "@/app/shell/mark";
import { type AccountActive, SnavAccount } from "@/app/shell/snav-account";
import { initialsOf, useSession } from "@/lib/session";

/**
 * The account shell (P2–P6): renders inside _app but outside $org — a minimal,
 * ctx-less top bar (no org crumb, no rail: nothing here belongs to Acme) with
 * the account snav beside the page.
 */
function activeFromPath(pathname: string): AccountActive {
  if (pathname.includes("/security")) return "security";
  if (pathname.includes("/sessions")) return "sessions";
  if (pathname.includes("/tokens")) return "tokens";
  if (pathname.includes("/notifications")) return "notifications";
  return "profile";
}

function AccountShell() {
  const session = useSession();
  const location = useLocation();

  return (
    <div className="flex h-screen flex-col bg-canvas">
      <header className="ctx">
        <Mark />
        <span className="text-12p5 font-medium">
          {session?.name ?? "You"} <span className="csep">/</span>{" "}
          <span className="text-ink3">Account settings</span>
        </span>
        <span className="flex-1" />
        <span className="uav" title={session?.name}>
          {session ? initialsOf(session.name) : "?"}
        </span>
      </header>
      <div className="fbody">
        <SnavAccount active={activeFromPath(location.pathname)} />
        <Outlet />
      </div>
    </div>
  );
}

export const Route = createFileRoute("/_app/account")({
  component: AccountShell,
});
