import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Pill } from "@/design-system/pill";

/**
 * P4 · Account · Sessions — everywhere you're signed in.
 * openapi.yaml has no sessions endpoint (list/revoke) — the rows are the P4
 * frame's canon and every revoke action states that gap (finding).
 */

const REVOKE_REASON = "Session revocation needs a sessions endpoint the spec lacks (finding)";

function SessionsPage() {
  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead title="Sessions" sub="Everywhere you're signed in right now">
          <Btn variant="dgr" disabled disabledReason={REVOKE_REASON}>
            Sign out all other sessions
          </Btn>
        </Pghead>

        <div className="tblwrap">
          <table className="tbl">
            <thead>
              <tr>
                <th>Session</th>
                <th>Location</th>
                <th>Signed in</th>
                <th>Last active</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>
                  <span className="flex items-center gap-2">
                    <b>Chrome · macOS</b>
                    <Pill tone="st">this session</Pill>
                  </span>
                </td>
                <td className="text-ink2">Bengaluru, IN</td>
                <td className="text-ink2">today 09:12</td>
                <td className="text-ink2">now</td>
                <td className="text-right text-ink3">—</td>
              </tr>
              <tr>
                <td>
                  <span className="flex items-center gap-2">
                    <b>steloit CLI · laptop</b>
                    <Pill tone="mut">device token</Pill>
                  </span>
                </td>
                <td className="text-ink2">Bengaluru, IN</td>
                <td className="text-ink2">3 d ago 14:01</td>
                <td className="text-ink2">today</td>
                <td className="text-right">
                  <Btn variant="gh" className="text-err" disabled disabledReason={REVOKE_REASON}>
                    Revoke
                  </Btn>
                </td>
              </tr>
              <tr>
                <td>
                  <b>Claude iOS app</b>
                </td>
                <td className="text-ink2">Bengaluru, IN</td>
                <td className="text-ink2">2 w ago</td>
                <td className="text-ink2">yesterday</td>
                <td className="text-right">
                  <Btn variant="gh" className="text-err" disabled disabledReason={REVOKE_REASON}>
                    Revoke
                  </Btn>
                </td>
              </tr>
              <tr className="bg-warn-tint">
                <td>
                  <span className="flex items-center gap-2">
                    <b>Chrome · Windows</b>
                    <Pill tone="warn">new · unrecognized?</Pill>
                  </span>
                </td>
                <td className="text-ink2">Mumbai, IN</td>
                <td className="text-ink2">yesterday 21:04</td>
                <td className="text-ink2">yesterday</td>
                <td className="text-right">
                  <Btn variant="dgr" disabled disabledReason={REVOKE_REASON}>
                    Revoke & secure…
                  </Btn>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <Card className="flex max-w-[640px] flex-col gap-2 p-4">
          <p className="text-11p5 leading-relaxed text-ink2">
            Revoke & secure revokes the session, forces a password change, and regenerates recovery
            codes in one flow.
          </p>
          <p className="mono text-10p5 text-ink3">
            sessions also die when an org removes you — G6's promise, kept here
          </p>
        </Card>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/account/sessions")({
  component: SessionsPage,
});
