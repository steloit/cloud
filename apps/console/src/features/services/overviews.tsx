import { useMutation } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Copybit } from "@/design-system/copybit";
import { Dot, Pill } from "@/design-system/pill";
import { type Binding, errorMessage, rollbackDeploymentMutation, type Service } from "@/lib/api";
import { fmtMoney } from "@/lib/fmt";
import {
  ActRow,
  ConnectPanel,
  type ConsumerRow,
  FloorStrip,
  RightNow,
  type VitalCell,
  VitalsStrip,
} from "./overview-zones";

/**
 * Per-product Overview content (frames S1–S6) — the shared grammar's zones
 * 2–5 for each product. Telemetry values are frame-fixed canon (metrics
 * endpoints are env-scoped; per-service panes land with D9/D13); costs and
 * bindings are live data.
 */

export interface OverviewCtx {
  svc: Service;
  org: string;
  project: string;
  env: string;
  projectTotalCents?: number;
  /** Bindings where this service is the target. */
  consumers: Binding[];
  /** Bindings where this service is the source. */
  uses: Binding[];
  nameOf: (serviceId: string) => string;
}

function costCell(svc: Service, projectTotalCents?: number, note?: string): VitalCell {
  return {
    label: "Cost",
    mono: true,
    value: `${fmtMoney(svc.monthly_estimate_cents ?? 0)}/mo`,
    note:
      note ??
      `of ${projectTotalCents !== undefined ? fmtMoney(projectTotalCents) : "…"} project · details`,
  };
}

function scopeLabel(scope: Binding["scope"]): string {
  return scope === "read_only" ? "read-only" : "read-write";
}

// --------------------------------------------------------------------------
// S1 · Valkey — calm state: keyspace shape as the Right-now panel
// --------------------------------------------------------------------------
export function ValkeyOverview({
  svc,
  org,
  project,
  env,
  projectTotalCents,
  consumers,
  nameOf,
}: OverviewCtx) {
  const rows: ConsumerRow[] = consumers.map((b) => ({
    icon: "s-globe",
    name: nameOf(b.source_id),
    pill: { tone: "mut", label: scopeLabel(b.scope) },
    note: "rotated 12 d",
  }));

  return (
    <>
      <VitalsStrip
        cells={[
          { label: "Hit ratio", value: "98.2%", note: "▼ −0.1 pt" },
          { label: "Ops / sec", value: "3.4k", note: <span className="text-ok">▲ +4%</span> },
          { label: "Memory", value: "612 MB / 1 GB", note: "▲ +18 MB" },
          {
            label: "Evictions",
            value: "0 / h",
            note: <span className="text-ok">→ none this week</span>,
          },
          costCell(svc, projectTotalCents),
        ]}
      />
      <div className="flex gap-3.5">
        <RightNow mood="Activity">
          <p className="text-12 text-ink2">
            41.2k keys · read-heavy 10:1 — a shape that fits cache mode
          </p>
          <div className="flex h-2 overflow-hidden rounded-full" aria-hidden="true">
            <span className="bg-steel" style={{ width: "68%" }} />
            <span className="bg-ok" style={{ width: "22%" }} />
            <span className="bg-surface2" style={{ width: "10%" }} />
          </div>
          <div className="mono text-10p5 text-ink3">session:* 68% · cart:* 22% · other 10%</div>
          <FloorStrip>
            <span>p99 cmd 0.8 ms</span>
            <span>84 clients</span>
            <span>frag 1.03</span>
            <Pill tone="mut">backups n/a in cache mode — stated</Pill>
          </FloorStrip>
        </RightNow>
        <ConnectPanel
          envPills={["VALKEY_URL — injected · rotated"]}
          cli={<Copybit>{`steloit valkey cli ${svc.name}`}</Copybit>}
          consumersEyebrow={`Consumers · ${consumers.length} binding${consumers.length === 1 ? "" : "s"}`}
          consumers={rows}
          footer="Dashboards and humans use the read-only CLI — writes require an audited unlock."
        />
      </div>
      <ActRow
        nav={{ org, project, env, service: svc.name }}
        tiles={[
          {
            title: "Open CLI",
            sub: "read-only session",
            tab: "cli",
          },
          {
            title: "Data browser",
            sub: "keys & TTLs",
            tab: "data-browser",
          },
          {
            title: "Switch to Durable…",
            sub: "+$6 · stated restart",
            tab: "settings",
          },
          {
            title: "Flush…",
            sub: "unlock + confirm",
            danger: true,
            tab: "settings",
          },
        ]}
      />
    </>
  );
}

// --------------------------------------------------------------------------
// S2 · Object Storage — activity feed; usage-based cost cell
// --------------------------------------------------------------------------
export function StorageOverview({
  svc,
  org,
  project,
  env,
  projectTotalCents,
  consumers,
  nameOf,
}: OverviewCtx) {
  const rows: ConsumerRow[] = consumers.map((b) => ({
    icon: b.source_id.includes("worker") ? "s-worker" : "s-globe",
    name: nameOf(b.source_id),
    pill: {
      tone: "mut",
      label: b.scope === "read_only" ? "read" : "write",
    },
    note: b.scope === "read_only" ? "rotated 12 d" : "scoped to uploads/",
  }));

  return (
    <>
      <VitalsStrip
        cells={[
          { label: "Stored", value: "48.1 GB", note: "▲ +0.4 GB / d" },
          { label: "Objects", value: "1.21 M", note: "▲ +2.1k / d" },
          { label: "Egress · 30 d", value: "214 GB", note: "→ steady" },
          {
            label: "Requests · 30 d",
            value: "4.2 M",
            note: <span className="text-ok">▲ +6%</span>,
          },
          // S2's cost cell is usage-based (mtd); $9.14 is the frame value.
          {
            label: "Cost",
            mono: true,
            value: "$9.14 mtd",
            note: `of ${projectTotalCents !== undefined ? fmtMoney(projectTotalCents) : "…"} project · details`,
          },
        ]}
      />
      <div className="flex gap-3.5">
        <RightNow mood="Activity">
          <div className="flex flex-col gap-2 text-11p5 text-ink2">
            <div className="flex gap-2.5">
              <span className="mono text-ink3">14:02</span>
              <span>
                api wrote <span className="mono">products/img_88213_hero.webp</span> · 214 KB
              </span>
            </div>
            <div className="flex gap-2.5">
              <span className="mono text-ink3">03:00</span>
              <span>
                lifecycle <span className="mono">tmp/</span> removed 2,041 objects · 1.9 GB
                reclaimed
              </span>
            </div>
            <div className="flex gap-2.5">
              <span className="mono text-ink3">Jul 5</span>
              <span>1,204 signed URLs served · median expiry 15 min</span>
            </div>
          </div>
          <FloorStrip>
            <Pill tone="ok">nothing public</Pill>
            <span>soft delete 30 d</span>
            <span>2 lifecycle rules</span>
          </FloorStrip>
        </RightNow>
        <ConnectPanel
          envPills={["ASSETS — S3-compatible · scoped creds"]}
          cli={<Copybit>steloit storage cp ./hero.webp assets/products/</Copybit>}
          consumersEyebrow={`Consumers · ${consumers.length} bindings`}
          consumers={rows}
          footer="End users get expiring signed URLs — nothing is world-readable by accident."
        />
      </div>
      <ActRow
        nav={{ org, project, env, service: svc.name }}
        tiles={[
          {
            title: "Object browser",
            sub: "prefixes & preview",
            tab: "objects",
          },
          {
            title: "Upload objects",
            sub: "drag or CLI",
            disabledReason: "No object-upload endpoint in the spec (finding)",
          },
          {
            title: "New lifecycle rule",
            sub: "dry-run first",
            tab: "lifecycle",
          },
          {
            title: "Request public CDN…",
            sub: "routed to an Admin",
            disabledReason:
              "Public access is a policy-gated request (D16) — no public-access endpoint in the spec (finding)",
          },
        ]}
      />
    </>
  );
}

// --------------------------------------------------------------------------
// S3 · Queue — DLQ attention card; delivery contract in chips
// --------------------------------------------------------------------------
export function QueueOverview({
  svc,
  org,
  project,
  env,
  projectTotalCents,
  consumers,
  nameOf,
}: OverviewCtx) {
  const rows: ConsumerRow[] = consumers.map((b) => ({
    icon: b.source_id.includes("worker") ? "s-worker" : "s-globe",
    name: nameOf(b.source_id),
    pill: {
      tone: "mut",
      label: nameOf(b.source_id) === "api" ? "publish" : "consume",
    },
    note: nameOf(b.source_id) === "api" ? "order.created · order.paid" : "concurrency 8",
  }));

  return (
    <>
      <VitalsStrip
        cells={[
          { label: "Depth", value: "12", note: "→ normal" },
          { label: "Throughput", value: "42/min", note: <span className="text-ok">▲ +9%</span> },
          { label: "Ack p95", value: "210 ms", note: "→ flat" },
          {
            label: "Dead letters",
            value: "2",
            tone: "warn",
            note: <span className="text-warn">▲ since 14:03</span>,
          },
          costCell(svc, projectTotalCents),
        ]}
      />
      <div className="flex gap-3.5">
        <RightNow mood="Needs attention">
          <div className="rounded-lg border border-warn/40 bg-warn-tint/40 p-3">
            <div className="flex items-baseline gap-2.5">
              <b className="text-12p5 text-warn">● 2 messages dead-lettered</b>
              <span className="mono text-10p5 text-ink3">14:03 · one minute after deploy #142</span>
            </div>
            <div className="logwell mt-2">
              order.paid · TypeError: receiptUrl undefined · payload has{" "}
              <span className="lv-e">"receipt": null</span>
            </div>
            <div className="mt-2.5 flex gap-2.5">
              <Link
                to="/$org/$project/svc/$service/messages"
                params={{ org, project, service: svc.name }}
                search={{ env }}
              >
                <Btn variant="p">View dead letters</Btn>
              </Link>
              <Btn variant="s" disabled disabledReason="No redrive endpoint in the spec (finding)">
                Redrive after the fix
              </Btn>
            </div>
          </div>
          <FloorStrip>
            <span>consumers healthy</span>
            <span>oldest ready msg 41 s</span>
            <span>1 schedule · next in 11 h</span>
          </FloorStrip>
        </RightNow>
        <ConnectPanel
          envPills={["QUEUE_JOBS_URL — injected · rotated"]}
          cli={<Copybit>{`steloit queue send ${svc.name} --type order.created`}</Copybit>}
          consumersEyebrow={`Producers & consumers · ${consumers.length} bindings`}
          consumers={rows}
          footer="The delivery contract is immutable after create — consumers were built against it."
        />
      </div>
      <ActRow
        nav={{ org, project, env, service: svc.name }}
        tiles={[
          {
            title: "Open messages",
            sub: "ready · in-flight · DLQ",
            tab: "messages",
          },
          {
            title: "Redrive DLQ (2)",
            sub: "after the fix ships",
            disabledReason: "No redrive endpoint in the spec (finding)",
          },
          {
            title: "New schedule (U4)",
            sub: "missed runs alert",
            tab: "schedules",
          },
          {
            title: "Purge…",
            sub: "typed name + reason",
            danger: true,
            tab: "settings",
          },
        ]}
      />
    </>
  );
}

// --------------------------------------------------------------------------
// S4 · Web service — live deploy + regression; rollback at hand
// --------------------------------------------------------------------------
export function WebOverview({
  svc,
  org,
  project,
  env,
  projectTotalCents,
  uses,
  nameOf,
}: OverviewCtx) {
  const rollback = useMutation(rollbackDeploymentMutation());
  const doRollback = () =>
    rollback.mutate(
      { path: { dep: "dep_142" } },
      {
        onSuccess: () => toast.success("Rollback started — api → #141 (~40s)"),
        onError: (err) => toast.error(errorMessage(err)),
      },
    );

  const usePills = uses.map((b) => {
    const envVar = Object.keys(b.env_vars ?? {})[0] ?? "URL";
    return `${envVar} ← ${nameOf(b.target_id)} · ${b.scope === "read_only" ? "ro" : "rw"}`;
  });

  return (
    <>
      <VitalsStrip
        cells={[
          {
            label: "p95 latency",
            value: "812 ms",
            tone: "warn",
            note: <span className="text-warn">▲ +540 vs prev hour</span>,
          },
          { label: "Requests", value: "214/s", note: <span className="text-ok">▲ +11%</span> },
          { label: "5xx rate", value: "0.4%", note: "→ flat" },
          { label: "Instances", value: "3", note: "autoscale 2–6 · cpu 41%" },
          costCell(svc, projectTotalCents),
        ]}
      />
      <div className="flex gap-3.5">
        <RightNow mood="Needs attention">
          <div className="rounded-lg border border-warn/40 bg-warn-tint/40 p-3">
            <div className="flex items-center gap-2.5">
              <Pill tone="st">#142</Pill>
              <b className="text-12p5">feat: gift cards at checkout</b>
              <span className="mono text-10p5 text-ink3">priya · 13:58 · 8f31c02</span>
            </div>
            <p className="mt-1.5 text-11p5 leading-relaxed text-ink2">
              p95 crossed 800 ms at 14:02 — diagnosis points at the database, not this code.
            </p>
            <div className="mt-2.5 flex gap-2.5">
              <Link
                to="/$org/$project/svc/$service/insights"
                params={{ org, project, service: "db-main" }}
                search={{ env }}
              >
                <Btn variant="p">View proposal prp_7c31a2</Btn>
              </Link>
              <Btn
                variant="s"
                onClick={doRollback}
                disabled={rollback.isPending}
                disabledReason="Rolling back…"
              >
                Roll back to #141
              </Btn>
            </div>
          </div>
          <FloorStrip>
            <span className="mono">GET /healthz</span>
            <Pill tone="ok">200 · 12 ms</Pill>
            <span>zero-downtime rollouts</span>
            <span>previews per PR</span>
          </FloorStrip>
        </RightNow>
        <ConnectPanel
          eyebrow="Uses · via bindings"
          envPills={usePills}
          cli={<Copybit>curl -s https://api.acme-store.com/healthz</Copybit>}
          consumersEyebrow="Serves"
          consumers={[
            {
              icon: "s-globe",
              name: "api.acme-store.com",
              pill: { tone: "ok", label: "TLS · renews 61 d" },
              note: "primary",
            },
            {
              icon: "s-globe",
              name: "*.preview.steloit.app",
              pill: { tone: "mut", label: "previews" },
              note: "per PR",
            },
          ]}
          footer="Injected and rotated by the platform — there is no .env to leak."
        />
      </div>
      <ActRow
        nav={{ org, project, env, service: svc.name }}
        tiles={[
          { title: "Roll back to #141", sub: "instant · image warm", onClick: doRollback },
          {
            title: "Open shell",
            sub: "audited · masked",
            tab: "shell",
          },
          {
            title: "Add domain",
            sub: "CNAME → auto-TLS",
            tab: "domains",
          },
          {
            title: "Adjust scaling",
            sub: "ceiling is a cost",
            tab: "scaling",
          },
        ]}
      />
    </>
  );
}

// --------------------------------------------------------------------------
// S5 · Worker — schedules and consumption as the Right-now panel
// --------------------------------------------------------------------------
export function WorkerOverview({
  svc,
  org,
  project,
  env,
  projectTotalCents,
  uses,
  nameOf,
}: OverviewCtx) {
  const usePills = uses.map((b) => {
    const target = nameOf(b.target_id);
    if (target === "jobs") return "jobs · consume · concurrency 8";
    return `${target} · ${scopeLabel(b.scope)}`;
  });

  return (
    <>
      <VitalsStrip
        cells={[
          { label: "Jobs / min", value: "38", note: <span className="text-ok">▲ +5%</span> },
          { label: "Success", value: "99.2%", note: "→ flat" },
          { label: "Ack p95", value: "210 ms", note: "→ flat" },
          {
            label: "Retries · 1h",
            value: "14",
            note: <span className="text-warn">▲ 2 → DLQ (see jobs)</span>,
          },
          costCell(svc, projectTotalCents),
        ]}
      />
      <div className="flex gap-3.5">
        <RightNow mood="Activity">
          <div className="flex flex-col gap-2 text-11p5">
            <div className="flex items-center gap-3">
              <b>receipts-daily</b>
              <span className="mono text-ink3">0 2 * * *</span>
              <Pill tone="ok">ok · 02:00 · 3m 41s</Pill>
              <span className="ml-auto text-10p5 text-ink3">next 11 h</span>
            </div>
            <div className="flex items-center gap-3">
              <b>cleanup-tmp</b>
              <span className="mono text-ink3">0 * * * *</span>
              <Pill tone="ok">ok · 14:00 · 12s</Pill>
              <span className="ml-auto text-10p5 text-ink3">next 37 m</span>
            </div>
          </div>
          <FloorStrip>
            <span>a missed schedule is an alert, not a silence</span>
            <Pill tone="mut">scale-to-zero off in production</Pill>
          </FloorStrip>
        </RightNow>
        <ConnectPanel
          eyebrow="Uses · via bindings"
          envPills={usePills}
          cli={<Copybit>{`steloit worker run ${svc.name} --job receipts-daily`}</Copybit>}
          consumersEyebrow="Triggers"
          consumers={[
            {
              icon: "s-queue",
              name: "jobs",
              pill: { tone: "mut", label: "queue · at-least-once" },
              note: "38/min",
            },
            {
              icon: "s-cron",
              name: "2 schedules",
              pill: { tone: "mut", label: "cron" },
              note: "IST · UTC shown",
            },
          ]}
          footer="Consumers must be idempotent — the contract is stated on the queue, honored here."
        />
      </div>
      <ActRow
        nav={{ org, project, env, service: svc.name }}
        tiles={[
          {
            title: "Run receipts-daily",
            sub: "manual · logged",
            disabledReason: "Manual runs land with a schedule-run endpoint the API lacks (finding)",
          },
          {
            title: "Open shell",
            sub: "audited · masked",
            tab: "shell",
          },
          {
            title: "Add schedule",
            sub: "cron + payload",
            tab: "schedules",
          },
          {
            title: "Adjust concurrency",
            sub: "backpressure aware",
            tab: "scaling",
          },
        ]}
      />
    </>
  );
}

// --------------------------------------------------------------------------
// S6 · PostgreSQL fresh instance — honest zeros, the "when amber appears" promise
// --------------------------------------------------------------------------
export function FreshPostgresOverview({
  svc,
  org,
  project,
  env,
  projectTotalCents,
  consumers,
  nameOf,
}: OverviewCtx) {
  const shape = (svc.shape ?? {}) as Record<string, unknown>;
  const rows: ConsumerRow[] = consumers.map((b) => ({
    icon: "s-worker",
    name: nameOf(b.source_id),
    pill: { tone: "mut", label: scopeLabel(b.scope) },
    note: "activated at ready · 14:32",
  }));

  return (
    <>
      <VitalsStrip
        cells={[
          { label: "p95 latency", value: "—", note: "no queries yet" },
          { label: "Transactions", value: "0/s", note: "waiting for first" },
          { label: "Connections", value: "1 / 200", note: "worker binding · idle" },
          {
            label: "Disk",
            value: `0.1 / ${String(shape.storage_gb ?? 10)} GB`,
            note: "→ autogrow on",
          },
          costCell(svc, projectTotalCents, "billing started 14:32"),
        ]}
      />
      <div className="flex gap-3.5">
        <RightNow mood="Activity">
          <div className="flex flex-1 flex-col items-center justify-center gap-2.5 rounded-lg border border-hair border-dashed p-6 text-center">
            <span className="glyph">
              <Dot tone="prov" />
            </span>
            <b className="text-13">Nothing yet</b>
            <p className="max-w-[420px] text-11p5 leading-relaxed text-ink3">
              This instance is 6 minutes old. First queries, slow-query findings, deploy markers and
              alerts will land here — the panel turns amber only when something needs you.
            </p>
            <div className="flex gap-2.5">
              <Link
                to="/$org/$project/svc/$service/sql-editor"
                params={{ org, project, service: svc.name }}
                search={{ env }}
              >
                <Btn variant="p">Run your first query</Btn>
              </Link>
              <Btn
                variant="s"
                disabled
                disabledReason="No import endpoint in the spec — the SQL editor is the first-query path (finding)"
              >
                Import data…
              </Btn>
            </div>
          </div>
          <FloorStrip>
            <span>first backup 02:00</span>
            <span>PITR window starts accruing now</span>
            <Pill tone="ok">binding active</Pill>
          </FloorStrip>
        </RightNow>
        <ConnectPanel
          envPills={["DATABASE_URL — injected · rotated"]}
          cli={<Copybit>{`steloit db connect ${svc.name}`}</Copybit>}
          consumersEyebrow={`Consumers · ${consumers.length} binding${consumers.length === 1 ? "" : "s"}`}
          consumers={rows}
          footer="No static secrets — credentials are minted per consumer and rotate on policy."
        />
      </div>
      <ActRow
        nav={{ org, project, env, service: svc.name }}
        tiles={[
          {
            title: "Open SQL editor",
            sub: "readonly role by default",
            tab: "sql-editor",
          },
          {
            title: "Import data",
            sub: "from dump or CSV",
            disabledReason:
              "No import endpoint in the spec — the SQL editor is the first-query path (finding)",
          },
          {
            title: "New branch",
            sub: "instant copy-on-write",
            disabledReason: "Branch creation needs the branches endpoint — spec change first",
          },
          {
            title: "New binding",
            sub: "least-privilege",
            tab: "bindings",
          },
        ]}
      />
    </>
  );
}

// --------------------------------------------------------------------------
// M1 · The adaptive rail, explained on internal-tools' tools-db overview:
// the rail is the project's shape — two products, so it shows exactly two.
// --------------------------------------------------------------------------
export function InternalToolsDbOverview({ svc, org, project }: OverviewCtx) {
  return (
    <>
      <VitalsStrip
        cells={[
          { label: "Connections", value: "14/100", note: "worker binding · idle" },
          { label: "Queries / sec", value: "64", note: "→ steady" },
          { label: "Storage", value: "2.1 GB", note: "→ autogrow on" },
          {
            label: "Cost",
            mono: true,
            value: `${fmtMoney(svc.monthly_estimate_cents ?? 0)}/mo`,
            note: "compute lives outside Steloit",
          },
          { label: "Status", value: "ready", note: "bindings still work" },
        ]}
      />
      <RightNow mood="Activity">
        <div className="flex flex-col gap-2">
          <b className="text-12p5">Why the rail is short here</b>
          <p className="text-12 leading-relaxed text-ink2">
            This project uses{" "}
            <b>PostgreSQL and Object Storage — so the rail shows exactly those two</b>, plus + to
            grow. The rail is never a catalog of everything Steloit sells; it is the shape of{" "}
            <i>this</i> application. No Valkey here, no Valkey icon — nothing to explain, nothing to
            dismiss.
          </p>
          <div className="flex items-center gap-3">
            <Link to="/$org/create" params={{ org }} search={{ env: "production" }}>
              <Btn variant="s">+ Add a product to this project</Btn>
            </Link>
            <Copybit>{`steloit valkey create cache --project ${project}`}</Copybit>
          </div>
        </div>
      </RightNow>
    </>
  );
}
