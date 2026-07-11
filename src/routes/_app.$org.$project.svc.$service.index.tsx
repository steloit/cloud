import { createFileRoute, Link } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { PRODUCT_ICON } from "@/app/shell/rail";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Icon } from "@/design-system/icon";
import { Stlab, statusDotTone } from "@/design-system/pill";
import { useProject } from "@/features/projects/hooks";
import { resolveService } from "@/features/services/gateway";
import { GatewayOverview } from "@/features/services/gateway-page";
import { useBindings, useServices } from "@/features/services/hooks";
import { ConnectPanel, VitalsStrip } from "@/features/services/overview-zones";
import {
  FreshPostgresOverview,
  InternalToolsDbOverview,
  type OverviewCtx,
  QueueOverview,
  StorageOverview,
  ValkeyOverview,
  WebOverview,
  WorkerOverview,
} from "@/features/services/overviews";
import { ProvisioningView } from "@/features/services/provisioning";
import type { Service } from "@/lib/api";
import { ageOf } from "@/lib/canon/now";
import { resolveEnvKey } from "@/lib/canon-env";
import { fmtMoney } from "@/lib/fmt";

/**
 * The shared Overview grammar (W4 · S1–S6): identity header, vitals with cost
 * last, Right now, Connect, Act. db-main renders the W4 amber state; a fresh
 * postgres instance renders S6; provisioning services render C4.
 */

function identityChips(svc: Service): string[] {
  const shape = (svc.shape ?? {}) as Record<string, unknown>;
  const chips: string[] = [];
  if (svc.product === "postgres") {
    chips.push(`PostgreSQL ${String(shape.version ?? "")}`);
    chips.push(`${cap(String(shape.size ?? ""))} · ${String(shape.storage_gb ?? "")} GB`);
  } else if (svc.product === "valkey") {
    chips.push(`Valkey · ${String(shape.mode ?? "")} · ${String(shape.eviction ?? "")}`);
    chips.push(`${String(shape.memory_mb ?? "")} MB`);
  } else if (svc.product === "web") {
    chips.push(`Web service · ${cap(String(shape.size ?? ""))}`);
    chips.push(`${String(shape.instances ?? 1)} instances · ${String(shape.health_check ?? "")}`);
  } else if (svc.product === "worker") {
    chips.push(`Worker · ${cap(String(shape.size ?? ""))}`);
    chips.push(`${String(shape.instances ?? 1)} instances`);
  } else if (svc.product === "queue") {
    chips.push(String(shape.delivery ?? "").replace(/_/g, "-"));
    if (shape.dlq) chips.push("DLQ on");
  } else if (svc.product === "storage") {
    chips.push(shape.public ? "public" : "private · signed URLs");
    if (shape.versioning) chips.push("versioning on");
  }
  if (svc.product === "ai-gateway") {
    // X1's identity line is a sentence, not chips — kept verbatim.
    return [
      "One endpoint in ecommerce / production",
      "4 models behind 3 routes",
      "a service with no fleet — config, not instances",
    ];
  }
  if (svc.region) chips.push(svc.region.replace("/", " · "));
  if (svc.product === "postgres") chips.push(`HA ${shape.ha ? "on" : "off"}`);
  if (svc.created_at) chips.push(ageOf(svc.created_at));
  return chips.filter((c) => c.trim().length > 1);
}

function cap(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

/** Status label per the S-frames: healthy for ready, the incident's suffixes for amber. */
function statusLabel(svc: Service): string {
  if (svc.product === "ai-gateway") return "healthy";
  if (svc.status === "ready")
    return svc.name === "db-reports" ? "ready · just provisioned" : "healthy";
  if (svc.status === "degraded") {
    if (svc.name === "jobs") return "2 dead letters";
    return "degraded · p95";
  }
  if (svc.status === "provisioning") return "provisioning · ~40s";
  return svc.status;
}

function headerActions(svc: Service) {
  switch (svc.product) {
    case "postgres":
      return (
        <Btn variant="s" disabled disabledReason="SQL Editor (D1) lands in Phase 3">
          Open SQL editor
        </Btn>
      );
    case "valkey":
      return (
        <Btn variant="s" disabled disabledReason="CLI Console (D6) lands in Phase 3">
          Open CLI
        </Btn>
      );
    case "storage":
      return (
        <Btn variant="s" disabled disabledReason="Uploads land with the Object Browser (D7)">
          Upload
        </Btn>
      );
    case "queue":
      return (
        <Btn variant="s" disabled disabledReason="Messages (D8) land in Phase 3">
          Open messages
        </Btn>
      );
    case "web":
      return (
        <Btn
          variant="s"
          disabled
          disabledReason="Use the act row's rollback — header rollback duplicates it"
        >
          <Icon id="s-undo" />
          Roll back to #141
        </Btn>
      );
    case "worker":
      return (
        <Btn variant="s" disabled disabledReason="Manual runs land with schedules in Phase 3">
          Run a job now
        </Btn>
      );
    case "ai-gateway":
      return (
        <Btn
          variant="s"
          disabled
          disabledReason="Model management needs a canon gateway service (X1 finding)"
        >
          Add model
        </Btn>
      );
    default:
      return null;
  }
}

/** W4 — db-main's amber overview (the incident's database vantage). */
function DbMainOverview(ctx: OverviewCtx) {
  const { svc, org, project, env, projectTotalCents, consumers, nameOf } = ctx;
  const shape = (svc.shape ?? {}) as Record<string, unknown>;
  const conns = shape.connections as { used?: number; max?: number } | undefined;

  return (
    <>
      <VitalsStrip
        cells={[
          { label: "p95 latency", value: "812 ms", tone: "warn", note: "▲ +540 vs prev hour" },
          { label: "Transactions", value: "1.4k/s", note: "▲ +6%" },
          {
            label: "Connections",
            value: `${conns?.used ?? 0} / ${conns?.max ?? 0}`,
            tone: "warn",
            note: `▲ ${Math.round(((conns?.used ?? 0) / Math.max(conns?.max ?? 1, 1)) * 100)}% of pool`,
          },
          {
            label: "Disk",
            value: `21.4 / ${String(shape.storage_gb ?? "")} GB`,
            note: "→ autogrow on",
          },
          {
            label: "Cost",
            mono: true,
            value: `${fmtMoney(svc.monthly_estimate_cents ?? 0)}/mo`,
            note: `of ${projectTotalCents !== undefined ? fmtMoney(projectTotalCents) : "…"} project · details`,
          },
        ]}
      />
      <div className="flex gap-3.5">
        <Card className="flex flex-1 flex-col gap-2.5 p-4">
          <Eyebrow className="text-warn">Needs attention</Eyebrow>
          <div className="flex items-baseline gap-2.5">
            <b className="text-[13px]">Slow query since deploy #142</b>
            <span className="mono text-[10.5px] text-ink3">14:02 · traced</span>
          </div>
          <div className="logwell">
            SELECT * FROM orders WHERE customer_id = $1 … · <span className="lv-w">642 ms</span> ·
            Seq Scan · 1.2 M rows
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
        <ConnectPanel
          envPills={["DATABASE_URL — injected · rotated"]}
          cli={<Copybit>{`steloit db connect ${svc.name}`}</Copybit>}
          consumersEyebrow={`Consumers · ${consumers.length} bindings`}
          consumers={consumers.map((b) => ({
            icon: b.source_id.includes("worker") ? ("s-worker" as const) : ("s-globe" as const),
            name: nameOf(b.source_id),
            pill: {
              tone: "mut" as const,
              label: b.scope === "read_only" ? "read-only" : "read-write",
            },
            note: nameOf(b.source_id) === "api" ? "141 conns · rotated 12 d" : "rotated 12 d",
          }))}
          footer="No static secrets — credentials are minted per consumer and rotate on policy."
        />
      </div>
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
          title="Restore (D5) lands in Phase 3"
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
          title="Bindings (D11/U2) land in Phase 3"
        >
          <b className="text-[12.5px]">New binding</b>
          <span className="text-[10.5px] text-ink3">least-privilege</span>
        </Card>
      </div>
    </>
  );
}

function overviewFor(ctx: OverviewCtx) {
  const { svc } = ctx;
  if (svc.status === "provisioning") return <ProvisioningView svc={svc} env={ctx.env} />;
  switch (svc.product) {
    case "postgres":
      if (ctx.project === "internal-tools" && svc.name === "tools-db") {
        // M1 — the adaptive-rail exemplar lives on this instance's overview.
        return <InternalToolsDbOverview {...ctx} />;
      }
      return svc.name === "db-main" ? (
        <DbMainOverview {...ctx} />
      ) : (
        <FreshPostgresOverview {...ctx} />
      );
    case "ai-gateway":
      return <GatewayOverview {...ctx} />;
    case "valkey":
      return <ValkeyOverview {...ctx} />;
    case "storage":
      return <StorageOverview {...ctx} />;
    case "queue":
      return <QueueOverview {...ctx} />;
    case "web":
      return <WebOverview {...ctx} />;
    case "worker":
      return <WorkerOverview {...ctx} />;
    default:
      return (
        <Card className="p-4 text-[12px] text-ink2">
          The {svc.product} overview lands in a later phase.
        </Card>
      );
  }
}

function ServiceOverview() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));
  const projectQuery = useProject(project);
  const svc = resolveService(services.data, service);
  const bindings = useBindings(svc?.id ?? "");
  const byId = new Map((services.data ?? []).map((s) => [s.id, s]));

  if (!svc) return <main className="main" />;

  const ctx: OverviewCtx = {
    svc,
    org,
    project,
    env,
    projectTotalCents: projectQuery.data?.monthly_cost_cents,
    consumers: (bindings.data ?? []).filter((b) => b.target_id === svc.id),
    uses: (bindings.data ?? []).filter((b) => b.source_id === svc.id),
    nameOf: (id) => byId.get(id)?.name ?? id.replace(/^svc_/, ""),
  };

  const provisioning = svc.status === "provisioning";

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          before={<Glyph id={PRODUCT_ICON[svc.product]} />}
          title={
            <span className="flex items-center gap-2.5">
              {svc.name}
              <Stlab tone={statusDotTone(svc.status)}>{statusLabel(svc)}</Stlab>
            </span>
          }
          sub={
            provisioning ? (
              <span className="mono">
                {identityChips(svc).slice(0, 2).join(" · ")} · {svc.id} · created just now
              </span>
            ) : (
              <span className="mono flex flex-wrap items-center gap-x-3 gap-y-1">
                {identityChips(svc).map((chip) => (
                  <span key={chip}>{chip}</span>
                ))}
                {svc.product !== "ai-gateway" ? (
                  <span className="inline-flex items-center gap-1">
                    {svc.id}
                    <Icon id="s-copy" className="h-2.5 w-2.5" />
                  </span>
                ) : null}
              </span>
            )
          }
        >
          {provisioning ? null : headerActions(svc)}
          <Btn variant="s">Docs</Btn>
        </Pghead>
        {overviewFor(ctx)}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/")({
  component: ServiceOverview,
});
