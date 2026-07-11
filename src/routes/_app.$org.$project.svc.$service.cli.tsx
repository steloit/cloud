import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Card } from "@/design-system/card";
import { Glyph } from "@/design-system/glyph";
import { Pill } from "@/design-system/pill";
import { useServices } from "@/features/services/hooks";
import { resolveEnvKey } from "@/lib/canon-env";

/**
 * D6 · Valkey CLI Console — 08-api has no CLI-exec endpoint although D6
 * requires one (finding: spec change needed before commands can run); the
 * transcript below is frame-fixed canon, read-only by construction.
 */

const RESPONSE_GET = '"{\\"cart\\":[\\"sku_2213\\",\\"sku_8841\\"],\\"exp\\":1751791220}"';

function CliPage() {
  const { project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));

  const svc = services.data?.find((s) => s.name === service || s.id === service);
  if (!svc) return <main className="main" />;
  if (svc.product !== "valkey") {
    return (
      <main className="main">
        <div className="pgpad">
          <Card className="p-4 text-12 text-ink2">
            The CLI Console is a Valkey surface — {svc.name} is a {svc.product} service.
          </Card>
        </div>
      </main>
    );
  }

  const prompt = `${svc.name}:${env}›`;

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        {/* Finding: the frame shows the bare "CLI Console" title — the design-system
            "Area · Thing" h1 grammar wins per the audit's P1 ruling. */}
        <Pghead
          before={<Glyph id="s-term" />}
          title="Valkey · CLI Console"
          sub={
            <span className="mono">
              {svc.name} · {env} · read-only session — writes require an explicit unlock
            </span>
          }
        >
          <span
            className="chip"
            title="Write unlock (logged, 15 min) needs a CLI-exec endpoint the spec lacks — finding"
          >
            read-only
            <span
              aria-hidden="true"
              className="relative h-[14px] w-6 shrink-0 rounded-full bg-steel"
            >
              <span className="absolute top-[2px] left-[12px] h-[10px] w-[10px] rounded-full bg-white" />
            </span>
          </span>
        </Pghead>

        <Card className="flex flex-col gap-2.5 p-4">
          {/* The transcript is `cache`'s frame-fixed canon — any other valkey
              instance opens a fresh session over an empty keyspace. */}
          {svc.name !== "cache" ? (
            <div className="logwell">
              <div className="t">
                # connected to {svc.name} · {env} · empty keyspace — try SCAN 0
              </div>
              <div>
                <span className="t">{prompt} </span>
                <span className="animate-pulse">▍</span>
              </div>
            </div>
          ) : (
            <div className="logwell">
              <div>
                <span className="t">{prompt} </span>GET session:usr_88412
              </div>
              <div>{RESPONSE_GET}</div>
              <div>
                <span className="t">{prompt} </span>TTL session:usr_88412
              </div>
              <div>(integer) 1187</div>
              <div>
                <span className="t">{prompt} </span>SCAN 0 MATCH session:* COUNT 100
              </div>
              <div className="lv-w">
                ⚠ large keyspace — sampled 100 of ~41k matches (full SCAN is throttled in
                production)
              </div>
              <div>
                <span className="t">{prompt} </span>DEL session:usr_88412
              </div>
              <div className="lv-e">
                ✗ blocked — session is read-only. Unlock writes (logged, 15 min) to run destructive
                commands.
              </div>
              <div>
                <span className="t">{prompt} </span>
                <span className="animate-pulse">▍</span>
              </div>
            </div>
          )}
          <div className="flex items-center gap-3 border-hair border-t pt-2.5">
            <Pill tone="mut">every command → audit log</Pill>
            <span className="text-10p5 text-ink3">
              ↑ history · tab completes keys · same session grammar as{" "}
              <span className="mono">steloit valkey cli</span>
            </span>
          </div>
        </Card>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/cli")({
  component: CliPage,
});
