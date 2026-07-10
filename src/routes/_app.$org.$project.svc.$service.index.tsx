import { createFileRoute, Link } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { PRODUCT_ICON, PRODUCT_LABEL } from "@/app/shell/rail";
import { Banner } from "@/design-system/banner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Icon } from "@/design-system/icon";
import { Metric } from "@/design-system/metric";
import { Pill, Stlab, statusDotTone } from "@/design-system/pill";
import { useProject } from "@/features/projects/hooks";
import { useBindings, useServices } from "@/features/services/hooks";
import type { Service } from "@/lib/api";
import { ageOf } from "@/lib/canon/now";
import { fmtMoney } from "@/lib/fmt";

/**
 * W4 · PostgreSQL Overview — the shared Overview grammar: identity chips,
 * vitals + cost (cost always the last cell), Right now, Connect, Act.
 * Non-postgres products render the same grammar from data; their product
 * frames (S1–S5) land in Phase 2.
 */

function identityChips(svc: Service): string[] {
  const shape = (svc.shape ?? {}) as Record<string, unknown>;
  const chips: string[] = [];
  if (svc.product === "postgres") {
    chips.push(`PostgreSQL ${String(shape.version ?? "")}`);
    chips.push(`${cap(String(shape.size ?? ""))} · ${String(shape.storage_gb ?? "")} GB`);
  } else if (svc.product === "valkey") {
    chips.push(`Valkey · ${String(shape.mode ?? "")} mode`);
    chips.push(`${String(shape.memory_mb ?? "")} MB · ${String(shape.eviction ?? "")}`);
  } else if (svc.product === "web" || svc.product === "worker") {
    chips.push(`${cap(String(shape.size ?? ""))} · ${String(shape.instances ?? 1)} instances`);
  } else if (svc.product === "queue") {
    chips.push(`${String(shape.delivery ?? "")}${shape.dlq ? " · DLQ on" : ""}`);
  } else if (svc.product === "storage") {
    chips.push(`${shape.public ? "public" : "private"}${shape.versioning ? " · versioned" : ""}`);
  }
  if (svc.region) chips.push(svc.region.replace("/", " · "));
  if (svc.product === "postgres") chips.push(`HA ${shape.ha ? "on" : "off"}`);
  if (svc.created_at) chips.push(ageOf(svc.created_at));
  return chips.filter((c) => c.trim() !== "·" && c.trim().length > 0);
}

function cap(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function statusSuffix(svc: Service): string {
  if (svc.name === "db-main" && svc.status === "degraded") return "degraded · p95";
  return svc.status;
}

function ServiceOverview() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(env);
  const projectQuery = useProject(project);
  const svc = services.data?.find((s) => s.name === service || s.id === service);
  const bindings = useBindings(svc?.id ?? "");
  const byId = new Map((services.data ?? []).map((s) => [s.id, s]));

  if (!svc) return <main className="main" />;

  const shape = (svc.shape ?? {}) as Record<string, unknown>;
  const conns = shape.connections as { used?: number; max?: number } | undefined;
  const isDbMain = svc.name === "db-main";
  const isPostgres = svc.product === "postgres";
  const consumers = (bindings.data ?? []).filter((b) => b.target_id === svc.id);
  const projectTotal = projectQuery.data?.monthly_cost_cents;

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          before={<Glyph id={PRODUCT_ICON[svc.product]} />}
          title={
            <span className="flex items-center gap-2.5">
              {svc.name}
              <Stlab tone={statusDotTone(svc.status)}>{statusSuffix(svc)}</Stlab>
            </span>
          }
          sub={
            <span className="mono flex flex-wrap items-center gap-x-3 gap-y-1">
              {identityChips(svc).map((chip) => (
                <span key={chip}>{chip}</span>
              ))}
              <span className="inline-flex items-center gap-1">
                {svc.id}
                <Icon id="s-copy" className="h-2.5 w-2.5" />
              </span>
            </span>
          }
        >
          {isPostgres ? (
            <Btn variant="s" disabled disabledReason="SQL Editor (D1) lands in Phase 2">
              Open SQL editor
            </Btn>
          ) : null}
          <Btn variant="s">Docs</Btn>
        </Pghead>

        {!isPostgres ? (
          <Banner tone="default">
            The {PRODUCT_LABEL[svc.product]} product page (S-family) lands in Phase 2 — this is the
            shared Overview grammar over live data.
          </Banner>
        ) : null}

        {/* Vitals strip — cost is always the last cell. db-main's telemetry is
            frame-fixed canon (W4); metrics endpoints land with Observe. */}
        {isDbMain ? (
          <div className="grid grid-cols-5 gap-3">
            <Metric label="p95 latency" value="812 ms" tone="warn" note="▲ +540 vs prev hour" />
            <Metric label="Transactions" value="1.4k/s" note="▲ +6%" />
            <Metric
              label="Connections"
              value={`${conns?.used ?? 0} / ${conns?.max ?? 0}`}
              tone="warn"
              note={`▲ ${Math.round(((conns?.used ?? 0) / Math.max(conns?.max ?? 1, 1)) * 100)}% of pool`}
            />
            <Metric
              label="Disk"
              value={`21.4 / ${String(shape.storage_gb ?? "")} GB`}
              note="→ autogrow on"
            />
            <Metric
              label="Cost"
              mono
              value={`${fmtMoney(svc.monthly_estimate_cents ?? 0)}/mo`}
              note={`of ${projectTotal !== undefined ? fmtMoney(projectTotal) : "…"} project · details`}
            />
          </div>
        ) : (
          <div className="grid grid-cols-5 gap-3">
            <Metric
              label="Status"
              value={svc.status}
              tone={svc.status === "degraded" ? "warn" : undefined}
              note="metrics appear with traffic"
            />
            <Metric
              label="Cost"
              mono
              value={`${fmtMoney(svc.monthly_estimate_cents ?? 0)}/mo`}
              note={`of ${projectTotal !== undefined ? fmtMoney(projectTotal) : "…"} project`}
            />
          </div>
        )}

        <div className="flex gap-3.5">
          {isDbMain ? (
            <Card className="flex flex-1 flex-col gap-2.5 p-4">
              <Eyebrow className="text-warn">Needs attention</Eyebrow>
              <div className="flex items-baseline gap-2.5">
                <b className="text-[13px]">Slow query since deploy #142</b>
                <span className="mono text-[10.5px] text-ink3">14:02 · traced</span>
              </div>
              <div className="logwell">
                SELECT * FROM orders WHERE customer_id = $1 … · <span className="lv-w">642 ms</span>{" "}
                · Seq Scan · 1.2 M rows
              </div>
              <div className="flex gap-2.5">
                <Link
                  to="/$org/$project/svc/$service/insights"
                  params={{ org, project, service: svc.name }}
                  search={{ env }}
                >
                  <Btn variant="p">View proposal prp_7c31a2</Btn>
                </Link>
                <Link
                  to="/$org/$project/svc/$service/insights"
                  params={{ org, project, service: svc.name }}
                  search={{ env }}
                >
                  <Btn variant="s">Query Insights</Btn>
                </Link>
              </div>
              <div className="flex gap-5 border-hair border-t pt-2.5 text-[11px] text-ink3">
                <span>
                  <b className="text-ink1">4</b> branches
                </span>
                <span>
                  last backup <b className="text-ok">verified · 02:00</b>
                </span>
                <span>PITR window 7 d</span>
                <span>WAL lag 0.8 s</span>
              </div>
            </Card>
          ) : (
            <Card className="flex flex-1 flex-col gap-2.5 p-4">
              <Eyebrow>Right now</Eyebrow>
              <p className="text-[12px] text-ink2">
                {svc.status === "ready"
                  ? "Calm — no attention items."
                  : svc.status === "degraded"
                    ? "Needs attention — see the project overview's attention rail."
                    : "Provisioning — billing starts at ready."}
              </p>
            </Card>
          )}

          <Card className="flex w-[380px] shrink-0 flex-col gap-2.5 p-4">
            <Eyebrow>Connect</Eyebrow>
            <div className="flex flex-wrap items-center gap-2">
              {consumers[0]?.env_vars
                ? Object.keys(consumers[0].env_vars).map((envVar) => (
                    <span key={envVar} className="bindpill">
                      <Icon id="s-bind" />
                      {envVar} — injected · rotated
                    </span>
                  ))
                : null}
            </div>
            {isPostgres ? <Copybit>{`steloit db connect ${svc.name}`}</Copybit> : null}
            <Eyebrow>Consumers · {consumers.length} bindings</Eyebrow>
            <div className="flex flex-col gap-1.5">
              {consumers.map((b) => (
                <div key={b.id} className="flex items-center gap-2 text-[11.5px]">
                  <b>{byId.get(b.source_id)?.name ?? b.source_id}</b>
                  <Pill tone="mut">{b.scope === "read_only" ? "read-only" : "read-write"}</Pill>
                  <span className="ml-auto text-[10.5px] text-ink3">
                    {b.status === "active" ? "rotated 12 d" : "issued · activates at ready"}
                  </span>
                </div>
              ))}
            </div>
            <p className="border-hair border-t pt-2 text-[10.5px] leading-relaxed text-ink3">
              No static secrets — credentials are minted per consumer and rotate on policy.
            </p>
          </Card>
        </div>

        {isPostgres ? (
          <div className="grid grid-cols-4 gap-3">
            <Link
              to="/$org/$project/svc/$service/branches"
              params={{ org, project, service: svc.name }}
              search={{ env }}
            >
              <Card className="flex h-full flex-col gap-1 p-3.5 hover:border-ink3">
                <b className="text-[12.5px]">New branch</b>
                <span className="text-[10.5px] text-ink3">instant copy-on-write</span>
              </Card>
            </Link>
            <Card
              className="flex flex-col gap-1 p-3.5 opacity-55"
              title="Restore (D5) lands in Phase 2"
            >
              <b className="text-[12.5px]">Restore to branch…</b>
              <span className="text-[10.5px] text-ink3">never in place</span>
            </Card>
            <Link
              to="/$org/$project/svc/$service/insights"
              params={{ org, project, service: svc.name }}
              search={{ env }}
            >
              <Card className="flex h-full flex-col gap-1 p-3.5 hover:border-ink3">
                <b className="text-[12.5px]">Query Insights</b>
                <span className="text-[10.5px] text-ink3">find the 642 ms</span>
              </Card>
            </Link>
            <Card
              className="flex flex-col gap-1 p-3.5 opacity-55"
              title="Bindings (D11/U2) land in Phase 2"
            >
              <b className="text-[12.5px]">New binding</b>
              <span className="text-[10.5px] text-ink3">least-privilege</span>
            </Card>
          </div>
        ) : null}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/")({
  component: ServiceOverview,
});
