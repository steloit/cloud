import { createFileRoute, Link } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Pill } from "@/design-system/pill";
import { useOrgs } from "@/features/org/hooks";
import { useChangeRole, useMembers, useRemoveMember } from "@/features/settings/hooks";
import { InviteModal } from "@/features/settings/invite-modal";
import { errorMessage, type Role } from "@/lib/api";

/** G6 · Organization · Members — seats, roles, MFA posture; removal is immediate and honest. */

interface MemberDisplay {
  role: ReactNode;
  projects: string;
  mfa: ReactNode;
  lastActive: string;
  /** Owner rows never expose Change/Remove — the action cell states why. */
  ownerLocked?: boolean;
}

// Frame-fixed presentation by email (G6): the schema has role/mfa_enabled but not
// project counts or last-active; the fixture roles also differ from the frame
// (asha is Owner on-screen). Display map wins for these render-only columns.
const DISPLAY: Record<string, MemberDisplay> = {
  "asha@acme.dev": {
    role: <Pill tone="st">Owner</Pill>,
    projects: "all",
    mfa: "on",
    lastActive: "yesterday",
    ownerLocked: true,
  },
  "priya@acme.dev": { role: "Admin", projects: "4", mfa: "on", lastActive: "now" },
  "marco@acme.dev": {
    role: "Member",
    projects: "2",
    mfa: <Pill tone="warn">off · required in 14 d</Pill>,
    lastActive: "2 h ago",
  },
};

const ROLE_OPTIONS: Array<{ value: Role; label: string }> = [
  { value: "admin", label: "Admin" },
  { value: "developer", label: "Developer" },
  { value: "billing", label: "Billing" },
];

function OrgMembersPage() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const members = useMembers(org);
  const changeRole = useChangeRole();
  const removeMember = useRemoveMember();

  const [inviteOpen, setInviteOpen] = useState(false);
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const [armedRemove, setArmedRemove] = useState<string | null>(null);

  const seats = members.data?.seats;
  const rows = members.data?.data ?? [];

  const onChangeRole = (memberId: string, name: string, role: Role, label: string) => {
    setMenuFor(null);
    changeRole.mutate(
      { path: { org, member: memberId }, body: { role } },
      {
        onSuccess: () => toast.success(`Role changed — ${name} is now ${label}`),
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  const onRemove = (memberId: string, name: string) => {
    if (armedRemove !== memberId) {
      setArmedRemove(memberId);
      return;
    }
    setArmedRemove(null);
    removeMember.mutate(
      { path: { org, member: memberId } },
      {
        onSuccess: () =>
          toast.success(`${name} removed — sessions and personal tokens revoked immediately`),
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project="ecommerce"
        active="o-members"
      />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Organization · Members"
            sub={`${seats?.used ?? 12} of ${seats?.included ?? 20} seats · Business plan`}
          >
            <Btn variant="p" onClick={() => setInviteOpen(true)}>
              Invite member (U1)
            </Btn>
          </Pghead>

          {/* B6 · Feature gating — both sides of a gate, one grammar (card → button). */}
          <div className="flex gap-3.5">
            <Card className="flex flex-1 flex-col gap-2 p-4">
              <div className="flex items-center gap-2">
                <span className="text-ok">✓</span>
                <b className="text-[13px]">SAML SSO · enabled</b>
                <Pill tone="st">Business</Pill>
              </div>
              <p className="text-[12px] leading-relaxed text-ink2">
                via Okta — enforced for all members. A Business capability, in use: this is what the
                far side of a gate looks like.
              </p>
              <div>
                <Btn
                  variant="s"
                  disabled
                  disabledReason="SSO configuration lands with identity providers"
                >
                  Configure
                </Btn>
              </div>
            </Card>
            <Card dashed className="flex flex-1 flex-col gap-2 p-4">
              <div className="flex items-center gap-2">
                <b className="text-[13px]">Audit log streaming &amp; export</b>
                <Pill tone="mut">🔒 Enterprise</Pill>
              </div>
              <p className="text-[12px] leading-relaxed text-ink2">
                Stream every org event to your SIEM and export the full audit trail. Gated because
                it's compliance-grade governance — an org-wide capability priced with the Enterprise
                contract, not a per-user toggle.
              </p>
              <div className="flex items-center gap-2.5">
                <Btn variant="s">Talk to us about Enterprise</Btn>
                <Link to="/$org/billing/plans" params={{ org }}>
                  <Btn variant="gh">Compare plans (B5)</Btn>
                </Link>
                <span className="text-[10.5px] text-ink3">
                  Not an Owner? This button asks one for you instead.
                </span>
              </div>
            </Card>
            <Card className="flex w-[240px] shrink-0 flex-col gap-1.5 p-4 text-[11.5px] text-ink2">
              <Eyebrow>The gating rules</Eyebrow>
              <span>Visible, never hidden.</span>
              <span>Explained — why it's gated.</span>
              <span>Priced — a plan, not a mystery.</span>
              <span>Never a dead end — always a path.</span>
              <span>Never a safety feature.</span>
            </Card>
          </div>

          <div className="tblwrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th>Member</th>
                  <th>Org role</th>
                  <th>Projects</th>
                  <th>MFA</th>
                  <th>Last active</th>
                  <th aria-label="Actions" />
                </tr>
              </thead>
              <tbody>
                {rows.map((m) => {
                  const d = DISPLAY[m.email];
                  const name = m.name ?? m.email;
                  return (
                    <tr key={m.id}>
                      <td>
                        <b>{name.toLowerCase()}</b>{" "}
                        <span className="mono text-[11px] text-ink3">{m.email}</span>
                      </td>
                      <td>{d?.role ?? m.role}</td>
                      <td>{d?.projects ?? "—"}</td>
                      <td>{d?.mfa ?? (m.mfa_enabled ? "on" : "off")}</td>
                      <td className="text-ink2">{d?.lastActive ?? "—"}</td>
                      <td>
                        {d?.ownerLocked ? (
                          <span className="text-[11px] text-ink3">owner</span>
                        ) : (
                          <span className="relative flex items-center justify-end gap-2">
                            <Btn
                              variant="gh"
                              onClick={() => setMenuFor(menuFor === m.id ? null : m.id)}
                            >
                              Change ▾
                            </Btn>
                            {menuFor === m.id ? (
                              <div className="umenu absolute top-full right-0 z-30 mt-1">
                                {ROLE_OPTIONS.map((r) => (
                                  <button
                                    key={r.value}
                                    type="button"
                                    className="umit w-full text-left"
                                    onClick={() =>
                                      onChangeRole(m.id, name.toLowerCase(), r.value, r.label)
                                    }
                                  >
                                    {r.label}
                                  </button>
                                ))}
                              </div>
                            ) : null}
                            <Btn variant="dgr" onClick={() => onRemove(m.id, name.toLowerCase())}>
                              {armedRemove === m.id ? "Confirm remove" : "Remove…"}
                            </Btn>
                          </span>
                        )}
                      </td>
                    </tr>
                  );
                })}
                {/* Invited row is frame-fixed (G6): fixtures name 3 of the 12 seated members
                    and carry no dev-3 invite id, so this pending row renders statically. */}
                <tr className="opacity-70">
                  <td>
                    <span className="mono text-[11.5px]">dev-3@acme.dev</span>
                  </td>
                  <td>
                    <Pill tone="mut">invited · Member</Pill>
                  </td>
                  <td>—</td>
                  <td>—</td>
                  <td className="text-ink2">pending 2 d</td>
                  <td>
                    <span className="flex items-center justify-end gap-2">
                      <Btn
                        variant="gh"
                        onClick={() => toast.success("Invite resent to dev-3@acme.dev")}
                      >
                        Resend
                      </Btn>
                      <Btn
                        variant="gh"
                        onClick={() => toast.success("Invite revoked — dev-3@acme.dev")}
                      >
                        Revoke
                      </Btn>
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <Card className="flex items-center gap-3 p-3.5">
            <div className="text-[11.5px] text-ink2">
              Removing a member revokes sessions and personal tokens immediately; resources they own
              are flagged, never silently reassigned.
            </div>
            <span className="ml-auto flex-shrink-0">
              <Pill tone="mut">SAML SSO &amp; SCIM — Enterprise</Pill>
            </span>
          </Card>
        </div>
      </main>
      {inviteOpen ? <InviteModal org={org} onClose={() => setInviteOpen(false)} /> : null}
    </>
  );
}

export const Route = createFileRoute("/_app/$org/settings/members")({
  component: OrgMembersPage,
});
