import { createFileRoute, Link } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { Pghead } from "@/app/shell/pghead";
import { PRODUCT_ICON } from "@/app/shell/rail";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { EmptyRow } from "@/design-system/empty-state";
import { Icon } from "@/design-system/icon";
import { Dot } from "@/design-system/pill";
import { SkeletonRows } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { useEnvironments } from "@/features/projects/hooks";
import { useServices } from "@/features/services/hooks";
import type { Product } from "@/lib/api";
import { fmtMoney } from "@/lib/fmt";

/**
 * M3 · Environments parity matrix. NOTE: this route is not in 02-ia's URL map
 * — it is reached from the env crumb's "Manage environments"; a minimal
 * addition, surfaced as a finding rather than resolved silently.
 *
 * Cost cells: the M3 frame shows $202 / $5.10 / $0.96 while the fixtures
 * carry 19910 / 670 / 220 cents — fixtures win (ADR-026), rendered from data.
 */

const PRODUCT_ORDER: Product[] = ["postgres", "valkey", "storage", "queue", "web", "worker"];

type CellKind = "ok" | "warn-x2" | "scaled" | "branch" | "sus" | "sus-selected";

/** Presence cells per the M3 frame, by env name then product. */
const MATRIX: Record<string, Partial<Record<Product, CellKind>>> = {
  production: {
    postgres: "warn-x2",
    valkey: "ok",
    storage: "ok",
    queue: "ok",
    web: "ok",
    worker: "ok",
  },
  staging: {
    postgres: "scaled",
    valkey: "scaled",
    storage: "ok",
    queue: "scaled",
    web: "scaled",
    worker: "scaled",
  },
  "preview/pr-142": {
    postgres: "branch",
    valkey: "scaled",
    storage: "scaled",
    queue: "sus-selected",
    web: "scaled",
    worker: "sus",
  },
};

/** Frame-fixed per-environment descriptors (region line for production). */
const ENV_NOTE: Record<string, string> = {
  production: "home: aws · ap-south-1",
  staging: "nightly data refresh",
  "preview/pr-142": "auto-retires on PR close",
};

function cell(kind: CellKind | undefined): ReactNode {
  switch (kind) {
    case "ok":
      return <Dot tone="ok" />;
    case "warn-x2":
      return (
        <span className="inline-flex items-center gap-1">
          <Dot tone="warn" />
          <span className="mono text-10 text-warn">×2</span>
        </span>
      );
    case "scaled":
      return (
        <span className="inline-flex items-center gap-1">
          <Dot tone="ok" />
          <span className="mono text-10 text-ink3">S</span>
        </span>
      );
    case "branch":
      return <span className="mono text-12 text-ok">⎇</span>;
    case "sus":
      return <Dot tone="sus" />;
    case "sus-selected":
      return (
        <span className="inline-flex rounded-full p-0.5 ring-2 ring-steel">
          <Dot tone="sus" />
        </span>
      );
    default:
      return <Dot tone="sus" />;
  }
}

function EnvironmentsPage() {
  const { org, project } = Route.useParams();
  const { env } = Route.useSearch();
  const environments = useEnvironments(project);
  const services = useServices(env);

  const present = new Set((services.data ?? []).map((s) => s.product));
  const products = PRODUCT_ORDER.filter((p) => present.size === 0 || present.has(p));

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Environments"
          sub="Shape is the project's; presence and scale are each environment's. Every environment is fully isolated — own network, own credentials, own data."
        >
          {/* Wired: C8 exists at /$org/$project/new-env — the combined
              region/cell selector is the create surface for environments. */}
          <Link to="/$org/$project/new-env" params={{ org, project }}>
            <Btn variant="p">+ New environment</Btn>
          </Link>
        </Pghead>

        {/* Four-state grammar (16-qa) on the matrix — both queries feed it
            (environments are the rows, services the columns), so pending and
            error group across the pair. An empty env list is unreachable in
            canon — a project is born with production — but the row states it
            honestly rather than rendering a headerless void. */}
        {environments.isError || services.isError ? (
          <ApiFailureCard
            title="The parity matrix didn't load"
            error={environments.error ?? services.error}
            requestLine={`GET /projects/${project}/envs`}
            onRetry={() => {
              environments.refetch();
              services.refetch();
            }}
          />
        ) : (
          <div className="tblwrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th>Environment</th>
                  {products.map((p) => (
                    <th key={p}>
                      <Icon id={PRODUCT_ICON[p]} className="h-3.5 w-3.5" />
                    </th>
                  ))}
                  <th>Cost / mo</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {environments.isPending || services.isPending ? (
                  <SkeletonRows cols={products.length + 3} />
                ) : (environments.data ?? []).length === 0 ? (
                  <EmptyRow cols={products.length + 3}>
                    No environments — a project is born with production, so this state is
                    unreachable in canon
                  </EmptyRow>
                ) : (
                  (environments.data ?? []).map((e) => (
                    <tr key={e.id}>
                      <td>
                        <b className="mono text-12p5">{e.name}</b>
                        <span className="ml-2 text-10p5 text-ink3">{ENV_NOTE[e.name] ?? ""}</span>
                      </td>
                      {products.map((p) => (
                        <td key={p}>{cell(MATRIX[e.name]?.[p])}</td>
                      ))}
                      <td className="mono">{fmtMoney(e.monthly_cost_cents ?? 0)}</td>
                      <td className="text-right">
                        <Link
                          to="/$org/$project"
                          params={{ org, project }}
                          search={{ env: e.name }}
                        >
                          <Btn variant="gh">Open →</Btn>
                        </Link>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        )}

        <Card className="flex flex-col gap-2.5 p-4">
          <div className="flex items-center gap-2 text-12 font-semibold">
            <Dot tone="sus" /> jobs · not in preview/pr-142
          </div>
          <p className="text-11p5 leading-relaxed text-ink2">
            Exists in production, staging. Previews provision a subset to stay cheap — policy
            preview-minimal. Clicking the queue on the rail in this environment shows this state
            instead of hiding the icon.
          </p>
          <div>
            {/* Wired: adding the queue to pr-142 is a create (ADR-014) — the
                ONE create surface, pre-scoped to this env and product. */}
            <Link
              to="/$org/create"
              params={{ org }}
              search={{ type: "queue", env: "preview/pr-142" }}
            >
              <Btn variant="s">Add to this environment · +$0.40/mo</Btn>
            </Link>
          </div>
        </Card>

        <div className="grid grid-cols-2 gap-3.5">
          <Card className="flex flex-col gap-1 p-3.5 text-11 text-ink2">
            <span>
              <span className="text-ok">●</span> present · S = scaled down
            </span>
            <span>
              <span className="text-ink3">●</span> not in this environment
            </span>
            <span>
              <span className="text-ok">⎇</span> database branch, not a copy
            </span>
          </Card>
          <Card className="p-3.5 text-11 leading-relaxed text-ink2">
            Why a filter, not a container: navigating into environments would hide drift and double
            the tree. Here, switching environment re-renders the page you're on — and this matrix
            plus the promotion lane (DP1) make drift impossible to miss.
          </Card>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/environments")({
  validateSearch: (search: Record<string, unknown>): { env: string } => ({
    env: typeof search.env === "string" ? search.env : "production",
  }),
  component: EnvironmentsPage,
});
