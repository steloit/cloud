import { createFileRoute, Link } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Icon, type IconId } from "@/design-system/icon";
import { Pill, type PillTone } from "@/design-system/pill";

/** P3 · Account · Security & MFA — account-level protection, not org-level. */

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
          <Btn variant="s" disabled disabledReason="Password change flows land in Phase 5">
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
    </main>
  );
}

export const Route = createFileRoute("/_app/account/security")({
  component: SecurityPage,
});
