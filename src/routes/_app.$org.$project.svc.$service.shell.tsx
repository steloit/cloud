import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Pill } from "@/design-system/pill";
import { useServices } from "@/features/services/hooks";

/**
 * D20 · Web/Worker · Shell — an ephemeral, audited session. 08-api has no
 * shell/exec endpoint (the one true endpoint gap here) — the transcript below
 * is frame-fixed from the D20 gallery frame.
 */

function ShellPage() {
  const { service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(env);

  const svc = services.data?.find((s) => s.name === service || s.id === service);
  if (!svc) return <main className="main" />;
  if (svc.product !== "web" && svc.product !== "worker") {
    return (
      <main className="main">
        <div className="pgpad">
          <h1 className="h1">Shell</h1>
          <p className="hsub">
            Shell (D20) is a web/worker surface — {svc.name} is {svc.product}.
          </p>
        </div>
      </main>
    );
  }

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Shell"
          sub={
            <span className="mono">
              {svc.name} · {env} · ephemeral session on instance i-2 · everything typed here is
              logged
            </span>
          }
        >
          <Pill tone="mut">session ends on close · 30 min max</Pill>
        </Pghead>

        <div className="logwell">
          <div className="t"># connected to api · i-2 of 3 · read-only filesystem except /tmp</div>
          <div>
            <span className="t">app@api-i2:~$</span> node -v
          </div>
          <div>v20.14.0</div>
          <div>
            <span className="t">app@api-i2:~$</span> env | grep DATABASE
          </div>
          <div>DATABASE_URL=postgres://bnd_api_dbmain:••••••@db-main.internal:5432/store</div>
          <div className="lv-w">
            ⚠ secret values are masked in shell output and session logs — the process sees them, you
            don't need to
          </div>
          <div>
            <span className="t">app@api-i2:~$</span> curl -s localhost:3000/healthz
          </div>
          <div>{'{"status":"ok","deploy":"#142","uptime":"1h 12m"}'}</div>
          <div>
            <span className="t">app@api-i2:~$</span> <span className="animate-pulse">▊</span>
          </div>
        </div>

        <p className="text-11 leading-relaxed text-ink3">
          No SSH keys, no bastion — access is your console identity + role, sessions are audit
          events, and Developers get shells in non-production only unless policy says otherwise.
        </p>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/shell")({
  component: ShellPage,
});
