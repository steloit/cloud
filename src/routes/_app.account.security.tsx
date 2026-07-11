import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Icon, type IconId } from "@/design-system/icon";
import { Flabel, Inp } from "@/design-system/inp";
import { Modal, ModalHead } from "@/design-system/overlay";
import { Pill, type PillTone } from "@/design-system/pill";
import { cn } from "@/lib/utils";

/** P3 · Account · Security & MFA — account-level protection, not org-level. */

/**
 * Change password — the overlay exists; the verb is gated. No password
 * endpoint in the spec (finding), so Save carries the reason once the
 * fields validate; the reason ladder surfaces empty/mismatch first.
 * Requirement rows mirror the reset-password page (A4) — one grammar.
 */
function ChangePasswordModal({ onClose }: { onClose: () => void }) {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");

  const checks = [
    { label: "At least 12 characters", pass: next.length >= 12 },
    {
      label: "Contains a number and a symbol",
      pass: /\d/.test(next) && /[^a-zA-Z0-9]/.test(next),
    },
    { label: "Not a previously used password", pass: next.length > 0 },
  ];

  const reason = !current
    ? "Enter your current password"
    : checks.some((c) => !c.pass)
      ? "Meet every requirement below"
      : !confirm
        ? "Confirm the new password"
        : confirm !== next
          ? "Passwords don't match"
          : "No password endpoint in the spec (finding)";

  return (
    <Modal label="Change password" onClose={onClose}>
      <ModalHead
        title="Change password"
        sub="Changing it signs out every other session — this one stays."
      />
      <div>
        <Flabel htmlFor="pw-current">Current password</Flabel>
        <Inp
          id="pw-current"
          type="password"
          value={current}
          autoFocus
          onChange={(e) => setCurrent(e.target.value)}
          autoComplete="current-password"
        />
      </div>
      <div>
        <Flabel htmlFor="pw-next">New password</Flabel>
        <Inp
          id="pw-next"
          type="password"
          value={next}
          onChange={(e) => setNext(e.target.value)}
          autoComplete="new-password"
        />
      </div>
      <div>
        <Flabel htmlFor="pw-confirm">Confirm new password</Flabel>
        <Inp
          id="pw-confirm"
          type="password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          autoComplete="new-password"
        />
      </div>
      <ul className="flex flex-col gap-1 text-11p5">
        {checks.map((c) => (
          <li
            key={c.label}
            className={cn("flex items-center gap-1.5", c.pass ? "text-ok" : "text-ink3")}
          >
            <Icon id={c.pass ? "s-check" : "s-x"} className="h-[11px] w-[11px]" /> {c.label}
          </li>
        ))}
      </ul>
      <div className="flex justify-end gap-2">
        <Btn variant="s" onClick={onClose}>
          Cancel
        </Btn>
        <Btn variant="p" disabled disabledReason={reason}>
          Save new password
        </Btn>
      </div>
    </Modal>
  );
}

function FactorRow({
  icon,
  label,
  pill,
  pillTone,
  meta,
  action,
}: {
  icon: IconId;
  label: string;
  pill: string;
  pillTone: PillTone;
  meta?: string;
  action: React.ReactNode;
}) {
  return (
    <div className="flex items-center gap-3 border-hair border-t py-2.5 first:border-t-0">
      <Icon id={icon} className="h-[15px] w-[15px] text-ink2" />
      <span className="text-12p5 font-medium">{label}</span>
      <Pill tone={pillTone}>{pill}</Pill>
      {meta ? <span className="mono text-10p5 text-ink3">{meta}</span> : null}
      <span className="ml-auto">{action}</span>
    </div>
  );
}

function SecurityPage() {
  const [pwOpen, setPwOpen] = useState(false);
  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Security & MFA"
          sub="These protect you everywhere — they belong to your account, not to Acme"
        />

        <Card className="flex max-w-[640px] items-center gap-4 p-4">
          <div className="flex-1">
            <div className="text-13 font-semibold">Password</div>
            <div className="mt-1 text-11p5 text-ink3">
              last changed 2 months ago · changing it signs out every other session
            </div>
          </div>
          <Btn variant="s" onClick={() => setPwOpen(true)}>
            Change password…
          </Btn>
        </Card>

        <Card className="flex max-w-[640px] flex-col p-4">
          <div className="mb-2 flex items-center gap-2.5">
            <span className="text-13 font-semibold">Multi-factor authentication</span>
            <Pill tone="ok">enabled</Pill>
          </div>
          <FactorRow
            icon="s-shield"
            label="Authenticator app"
            pill="primary"
            pillTone="st"
            meta="enrolled 7 mo ago"
            action={
              <Btn
                variant="gh"
                className="text-err"
                disabled
                disabledReason="Blocked while mfa-required applies"
              >
                Remove…
              </Btn>
            }
          />
          <FactorRow
            icon="s-key"
            label="Security key / passkey"
            pill="none"
            pillTone="mut"
            action={
              <Btn variant="s" disabled disabledReason="Passkey enrollment lands in Phase 5">
                Add passkey
              </Btn>
            }
          />
          <FactorRow
            icon="s-doc"
            label="Recovery codes"
            pill="codes"
            pillTone="mut"
            meta="8 of 10 remaining"
            action={
              <Btn
                variant="s"
                disabled
                disabledReason="Recovery-code regeneration lands in Phase 5"
              >
                Regenerate… invalidates old
              </Btn>
            }
          />
          <p className="mt-2 border-hair border-t pt-3 text-11p5 leading-relaxed text-ink3">
            Org policy <span className="mono">mfa-required</span> makes MFA mandatory across Acme
            from Jul 20 — you're already covered. Removing your last factor is blocked while the
            policy applies.
          </p>
        </Card>

        <Card className="flex max-w-[640px] flex-col gap-2.5 p-4">
          <div className="text-13 font-semibold">Recent security events</div>
          <div className="flex items-baseline gap-2.5 text-11p5">
            <span className="mono text-10p5 text-ink3">yesterday</span>
            <span className="text-ink2">
              New session · Chrome · Mumbai, IN — that wasn't you?{" "}
              <Link to="/account/sessions" className="font-medium text-steel">
                Review sessions →
              </Link>
            </span>
          </div>
          <div className="flex items-baseline gap-2.5 text-11p5">
            <span className="mono text-10p5 text-ink3">2 mo</span>
            <span className="text-ink2">Password changed · all other sessions signed out</span>
          </div>
        </Card>
      </div>
      {pwOpen ? <ChangePasswordModal onClose={() => setPwOpen(false)} /> : null}
    </main>
  );
}

export const Route = createFileRoute("/_app/account/security")({
  component: SecurityPage,
});
