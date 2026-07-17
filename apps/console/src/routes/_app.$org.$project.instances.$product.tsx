import { createFileRoute, Link } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { PRODUCT_ICON, PRODUCT_LABEL } from "@/app/shell/rail";
import { Banner } from "@/design-system/banner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { EmptyRow } from "@/design-system/empty-state";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Pill, Stlab, statusDotTone } from "@/design-system/pill";
import { useServices } from "@/features/services/hooks";
import type { Product, Service } from "@/lib/api";
import { fmtMoney } from "@/lib/fmt";

/**
 * M2 (and the M4–M8 illustrative grammar) · Instance landing per product.
 * Canon today reaches ×2 on PostgreSQL only — that landing renders from live
 * services data with a frame-fixed display map; the other five products
 * render the M4–M8 illustrative two-row tables behind a banner.
 */

/** M2 · PostgreSQL — frame-fixed display strings merged with live data. */
const PG_DISPLAY: Record<
  string,
  {
    role: string;
    version: string;
    size: string;
    conns: string;
    connsWarn?: boolean;
    bindings: string;
  }
> = {
  "db-main": {
    role: "primary workload",
    version: "16.4",
    size: "21.4 GB",
    conns: "192/200",
    connsWarn: true,
    bindings: "api · worker · bi",
  },
  "db-reports": {
    role: "reporting · isolated",
    version: "16.4",
    size: "9.8 GB",
    conns: "31/100",
    bindings: "worker",
  },
};

interface IllustrativeCell {
  v: string;
  warn?: boolean;
}

interface IllustrativeRow {
  name: string;
  role: string;
  cells: IllustrativeCell[];
  status: { tone: "ok" | "warn"; label: string };
}

interface IllustrativeProduct {
  columns: string[];
  rows: IllustrativeRow[];
  rationale: string;
}

/** M4–M8 — per-product frame-fixed two-row tables and rationales. */
const ILLUSTRATIVE: Partial<Record<Product, IllustrativeProduct>> = {
  valkey: {
    columns: ["Mode", "Memory", "Hit rate", "Ops/s", "Cost / mo"],
    rows: [
      {
        name: "cache",
        role: "page & query cache",
        cells: [
          { v: "cache · allkeys-lru" },
          { v: "612 MB / 1 GB" },
          { v: "98.2%" },
          { v: "3.4k" },
          { v: "$22" },
        ],
        status: { tone: "ok", label: "healthy" },
      },
      {
        name: "sessions",
        role: "carts & sessions",
        cells: [
          { v: "cache · volatile-lru" },
          { v: "204 MB / 1 GB" },
          { v: "99.1%" },
          { v: "0.9k" },
          { v: "$22" },
        ],
        status: { tone: "ok", label: "healthy" },
      },
    ],
    rationale:
      "Different risk appetites deserve different instances: cache holds recomputable page fragments where eviction is free, while sessions holds carts where eviction logs users out. Separate instances mean separate memory pressure, separate eviction policies, and a flush on one never touches the other.",
  },
  storage: {
    columns: ["Access", "Objects", "Stored", "Egress / mo", "Cost / mo"],
    rows: [
      {
        name: "assets",
        role: "published set",
        cells: [
          { v: "private · signed" },
          { v: "1.21 M" },
          { v: "48.1 GB" },
          { v: "214 GB" },
          { v: "$9.14" },
        ],
        status: { tone: "ok", label: "healthy" },
      },
      {
        name: "uploads",
        role: "raw intake",
        cells: [{ v: "private" }, { v: "2.1 k" }, { v: "0.9 GB" }, { v: "2 GB" }, { v: "$0.42" }],
        status: { tone: "ok", label: "healthy" },
      },
    ],
    rationale:
      "uploads quarantines raw user intake (scanned, size-capped, lifecycle-drained) from assets, the published set the storefront serves. The worker promotes objects across the boundary — publishing is an explicit act, never a side effect of receiving.",
  },
  queue: {
    columns: ["Delivery", "Depth", "Dead letters", "Throughput", "Cost / mo"],
    rows: [
      {
        name: "jobs",
        role: "orders & receipts",
        cells: [
          { v: "at-least-once" },
          { v: "12" },
          { v: "2", warn: true },
          { v: "42/min" },
          { v: "$12" },
        ],
        status: { tone: "warn", label: "2 dead letters" },
      },
      {
        name: "emails",
        role: "transactional mail",
        cells: [{ v: "at-least-once" }, { v: "0" }, { v: "0" }, { v: "3/min" }, { v: "$6" }],
        status: { tone: "ok", label: "healthy" },
      },
    ],
    rationale:
      "Failure domains: a poison message in order processing must never delay password-reset emails. jobs and emails each carry their own delivery contract, retry policy, DLQ and alerts — congestion in one is invisible to the other.",
  },
  web: {
    columns: ["Domain", "Instances", "p95", "Deploy", "Cost / mo"],
    rows: [
      {
        name: "api",
        role: "customer traffic",
        cells: [
          { v: "api.acme-store.com" },
          { v: "3 · autoscale 2–6" },
          { v: "812 ms", warn: true },
          { v: "#142" },
          { v: "$61" },
        ],
        status: { tone: "warn", label: "degraded · p95" },
      },
      {
        name: "admin",
        role: "back-office",
        cells: [
          { v: "platform subdomain" },
          { v: "1 · fixed" },
          { v: "96 ms" },
          { v: "#3" },
          { v: "$14" },
        ],
        status: { tone: "ok", label: "healthy" },
      },
    ],
    rationale:
      "Blast radius and scaling shape: api serves customers and autoscales on traffic; admin serves nine teammates on a fixed single instance with read-only database credentials. A bad admin deploy can never take checkout down — and vice versa.",
  },
  worker: {
    columns: ["Consumes", "Jobs / min", "Success", "Concurrency", "Cost / mo"],
    rows: [
      {
        name: "worker",
        role: "orders & crons",
        cells: [{ v: "jobs · 2 schedules" }, { v: "38" }, { v: "99.2%" }, { v: "8" }, { v: "$22" }],
        status: { tone: "ok", label: "healthy" },
      },
      {
        name: "mailer",
        role: "transactional mail",
        cells: [
          { v: "emails · scale-to-zero" },
          { v: "2" },
          { v: "100%" },
          { v: "4" },
          { v: "$3" },
        ],
        status: { tone: "ok", label: "healthy" },
      },
    ],
    rationale:
      "Latency isolation: worker chews heavy order batches; mailer must send a reset email in seconds. Sharing one worker would let a batch backlog starve the urgent lane — separate workers, separate concurrency, separate scale-to-zero economics.",
  },
};

function isProduct(value: string): value is Product {
  return value in PRODUCT_LABEL;
}

function PostgresLanding({
  org,
  project,
  env,
  instances,
}: {
  org: string;
  project: string;
  env: string;
  instances: Service[];
}) {
  return (
    <>
      <Pghead
        before={<Glyph id="s-db" />}
        title={`PostgreSQL · instances in ${project} / ${env}`}
        sub="One instance → the rail opens it directly. Two or more → you land here, and the sidebar header switches."
      >
        <Link to="/$org/create" params={{ org }} search={{ env, type: "postgres" }}>
          <Btn variant="p">+ New PostgreSQL</Btn>
        </Link>
      </Pghead>

      <div className="tblwrap">
        <table className="tbl">
          <thead>
            <tr>
              <th>Instance</th>
              <th>Version</th>
              <th>Size</th>
              <th>Connections</th>
              <th>Bindings</th>
              <th>Cost / mo</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {instances.length === 0 ? (
              <EmptyRow cols={7}>
                No PostgreSQL instances in this environment yet — create one and it lands here with
                its version, bindings and cost.
              </EmptyRow>
            ) : null}
            {instances.map((svc) => {
              const display = PG_DISPLAY[svc.name];
              return (
                <tr key={svc.id}>
                  <td>
                    <Link
                      to="/$org/$project/svc/$service"
                      params={{ org, project, service: svc.name }}
                      search={{ env }}
                      className="flex items-center gap-2"
                    >
                      <b className="mono text-12p5">{svc.name}</b>
                      {display ? <Pill tone="mut">{display.role}</Pill> : null}
                    </Link>
                  </td>
                  <td className="mono">{display?.version ?? ""}</td>
                  <td className="mono">{display?.size ?? ""}</td>
                  <td className={display?.connsWarn ? "mono text-warn" : "mono"}>
                    {display?.conns ?? ""}
                  </td>
                  <td className="mono">{display?.bindings ?? ""}</td>
                  <td className="mono">{fmtMoney(svc.monthly_estimate_cents ?? 0)}</td>
                  <td>
                    <Stlab tone={statusDotTone(svc.status)}>
                      {svc.status === "ready" ? "ready" : svc.status}
                    </Stlab>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <Card className="p-4 text-11p5 leading-relaxed text-ink2">
        Both instances appear as separate nodes in the topology (W3) with their own bindings,
        backups and branches — instances are peers, never hidden inside each other. Names are yours;
        the platform never assumes "the database".
      </Card>

      <Card className="flex flex-col gap-2 p-4">
        <Eyebrow>Why a second instance, not a bigger first one</Eyebrow>
        <p className="text-12 leading-relaxed text-ink2">
          Reporting queries were competing with checkout traffic for the same connection pool.
          Isolating them into db-reports (fed by the worker) protects the primary — the same
          isolation logic environments give you, applied inside one environment.
        </p>
      </Card>
    </>
  );
}

function IllustrativeLanding({
  product,
  project,
  env,
  config,
}: {
  product: Product;
  project: string;
  env: string;
  config: IllustrativeProduct;
}) {
  return (
    <>
      <Pghead
        before={<Glyph id={PRODUCT_ICON[product]} />}
        title={`${PRODUCT_LABEL[product]} · instances in ${project} / ${env}`}
        sub="Instances are peers — same page grammar, separate credentials, separate failure domains. Click one, or switch anywhere via the ▾ in the sidebar header."
      />

      <Banner>Illustrative — canon today reaches ×2 on PostgreSQL only</Banner>

      <div className="tblwrap">
        <table className="tbl">
          <thead>
            <tr>
              <th>Instance</th>
              {config.columns.map((col) => (
                <th key={col}>{col}</th>
              ))}
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {config.rows.map((row) => (
              <tr key={row.name}>
                <td>
                  <span className="flex items-center gap-2">
                    <b className="mono text-12p5">{row.name}</b>
                    <Pill tone="mut">{row.role}</Pill>
                  </span>
                </td>
                {row.cells.map((cell) => (
                  <td key={cell.v} className={cell.warn ? "mono text-warn" : "mono"}>
                    {cell.v}
                  </td>
                ))}
                <td>
                  <Stlab tone={row.status.tone}>{row.status.label}</Stlab>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <Card className="flex flex-col gap-2 p-4">
        <Eyebrow>Why a second instance, not a bigger first one</Eyebrow>
        <p className="text-12 leading-relaxed text-ink2">{config.rationale}</p>
      </Card>
    </>
  );
}

function InstancesPage() {
  const { org, project, product } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(env);

  if (!isProduct(product)) {
    return (
      <main className="main">
        <div className="pgpad">
          {/* Finding: this fallback carried a bare "Instances" h1 — the "Area · Thing"
              grammar wants an honest Thing even when the product doesn't resolve. */}
          <h1 className="h1">Instances · unknown product</h1>
          <p className="hsub">
            No product named <span className="mono">{product}</span> — the rail lists the six.
          </p>
        </div>
      </main>
    );
  }

  const instances = (services.data ?? []).filter((s) => s.product === product);
  const illustrative = ILLUSTRATIVE[product];

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        {product === "postgres" ? (
          <PostgresLanding org={org} project={project} env={env} instances={instances} />
        ) : illustrative ? (
          <IllustrativeLanding
            product={product}
            project={project}
            env={env}
            config={illustrative}
          />
        ) : (
          <>
            <Pghead
              before={<Glyph id={PRODUCT_ICON[product]} />}
              title={`${PRODUCT_LABEL[product]} · instances in ${project} / ${env}`}
              sub="One instance → the rail opens it directly. Two or more → you land here, and the sidebar header switches."
            />
            <p className="text-11p5 text-ink3">
              No {PRODUCT_LABEL[product]} instances in this environment.
            </p>
          </>
        )}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/instances/$product")({
  validateSearch: (search: Record<string, unknown>): { env: string } => ({
    env: typeof search.env === "string" ? search.env : "production",
  }),
  component: InstancesPage,
});
