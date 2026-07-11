import { createFileRoute, Link } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Flabel, Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { useSession } from "@/lib/session";

/** P2 · Account · Profile — one identity across every organization. */

function ProfilePage() {
  const session = useSession();
  const name = session?.name ?? "Priya Sharma";
  const email = session?.email ?? "priya@acme.dev";

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead title="Profile" sub="Your identity across every organization you belong to" />

        <Card className="flex items-center gap-4 p-4">
          <span className="uav !h-14 !w-14 text-[18px]">PS</span>
          <div className="flex-1">
            <div className="text-13 font-semibold">{name}</div>
            <div className="mono mt-0.5 text-10p5 text-ink3">usr_2ka91f · joined Mar 2026</div>
          </div>
          <Btn variant="s" disabled disabledReason="Avatar upload lands in Phase 5">
            Change photo
          </Btn>
        </Card>

        <Card className="flex max-w-[560px] flex-col gap-3.5 p-4">
          <div>
            <Flabel htmlFor="pf-name">Full name</Flabel>
            <Inp id="pf-name" value={name} readOnly />
          </div>
          <div>
            <Flabel htmlFor="pf-email" aside={<Pill tone="ok">verified</Pill>}>
              Email
            </Flabel>
            <Inp id="pf-email" className="mono" value={email} readOnly />
          </div>
          <div className="flex items-center gap-2.5 border-hair border-t pt-3 text-11p5 text-ink3">
            <Pill tone="ok">MFA enabled · passkey + TOTP</Pill>
            <span>Last sign-in: today 09:02 · Bengaluru, IN</span>
            <Link to="/account/security" className="ml-auto text-11 font-medium text-steel">
              Manage in Security →
            </Link>
          </div>
        </Card>

        <div className="tblwrap">
          <table className="tbl">
            <thead>
              <tr>
                <th>Organization</th>
                <th>Role</th>
                <th>Joined</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>
                  <span className="flex items-center gap-2.5">
                    <span
                      className="cav flex h-[18px] w-[18px] items-center justify-center rounded-[5px] text-10 font-bold text-white"
                      style={{ background: "linear-gradient(135deg,#E36C4B,#B34A2E)" }}
                    >
                      A
                    </span>
                    Acme
                  </span>
                </td>
                <td>
                  <Pill tone="st">Admin</Pill>
                </td>
                <td className="mono text-11p5 text-ink2">Mar 2026</td>
                <td className="text-right">
                  <Btn
                    variant="gh"
                    className="text-err"
                    disabled
                    disabledReason="Leaving requires transferring owned resources — Phase 5"
                  >
                    Leave
                  </Btn>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <Card className="flex max-w-[560px] items-center gap-4 border-err/45 p-4">
          <div className="flex-1">
            <div className="text-13 font-semibold">Delete account</div>
            <div className="mt-1 text-11p5 leading-relaxed text-ink3">
              Requires leaving or transferring every organization first. Irreversible after the
              14-day grace period.
            </div>
          </div>
          <Btn
            variant="dgr"
            disabled
            disabledReason="Requires leaving or transferring every organization first — Phase 5"
          >
            Delete account…
          </Btn>
        </Card>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/account/profile")({
  component: ProfilePage,
});
