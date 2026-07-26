import { Link } from "@tanstack/react-router";
import { Icon } from "@/design-system/icon";
import { Nfoot, NitLink, Snav } from "./snav";

/**
 * Snav variant — the account plane (P2–P6): settings that belong to the
 * person, not to any organization.
 *
 * /account/sessions is not in 02-ia's route list, but the P4 frame exists —
 * minimal addition so Sessions has a home (finding).
 */
export type AccountActive = "profile" | "security" | "sessions" | "tokens" | "notifications";

export function SnavAccount({ active }: { active: AccountActive }) {
  return (
    <Snav>
      {/* The chevron reads as "back" — so it is one (audit: dead affordance). */}
      <Link to="/" className="snhead" aria-label="Back to the console">
        <Icon id="s-chev" className="h-3.5 w-3.5 rotate-180 text-ink3" />
        <div className="t">Account</div>
      </Link>
      <NitLink to="/account/profile" label="Profile" on={active === "profile"} />
      <NitLink to="/account/security" label="Security & MFA" on={active === "security"} />
      <NitLink to="/account/sessions" label="Sessions" count="3" on={active === "sessions"} />
      <NitLink to="/account/tokens" label="Personal tokens" count="2" on={active === "tokens"} />
      <NitLink to="/account/notifications" label="Notifications" on={active === "notifications"} />
      <Nfoot>
        <div className="px-1.5">These settings belong to you, not to any organization.</div>
      </Nfoot>
    </Snav>
  );
}
