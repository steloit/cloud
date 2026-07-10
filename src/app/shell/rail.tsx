import { Link } from "@tanstack/react-router";
import { Icon, type IconId } from "@/design-system/icon";
import type { Product, Service } from "@/lib/api";
import { initialsOf, useSession } from "@/lib/session";
import { cn } from "@/lib/utils";

/**
 * The icon rail (ADR-011/017): Home · Observe · Deploy, then the services
 * zone — the project's actual products, fleet entries carrying ×n badges —
 * then + (create) and the bottom cluster (gear · docs · avatar).
 * Environment switches never reshuffle the rail (ADR-013).
 */

export const PRODUCT_ICON: Record<Product, IconId> = {
  postgres: "s-db",
  valkey: "s-chip",
  storage: "s-bucket",
  queue: "s-queue",
  web: "s-globe",
  worker: "s-worker",
  "gpu-worker": "s-worker",
  "ai-gateway": "s-ai",
};

export const PRODUCT_LABEL: Record<Product, string> = {
  postgres: "PostgreSQL",
  valkey: "Valkey",
  storage: "Object Storage",
  queue: "Queue",
  web: "Web service",
  worker: "Worker",
  "gpu-worker": "GPU Worker",
  "ai-gateway": "AI Gateway",
};

export type RailActive =
  | { kind: "home" }
  | { kind: "observe" }
  | { kind: "deploy" }
  | { kind: "product"; product: Product }
  | { kind: "create" }
  | { kind: "settings" }
  | { kind: "none" };

interface RailProps {
  org: string;
  project?: string;
  env?: string;
  services: Service[];
  active: RailActive;
}

interface ProductEntry {
  product: Product;
  count: number;
  firstService: Service;
  degraded: boolean;
}

function productEntries(services: Service[]): ProductEntry[] {
  const byProduct = new Map<Product, Service[]>();
  for (const svc of services) {
    const group = byProduct.get(svc.product) ?? [];
    group.push(svc);
    byProduct.set(svc.product, group);
  }
  return [...byProduct.entries()].map(([product, group]) => {
    const first = group[0];
    if (!first) throw new Error("empty product group");
    return {
      product,
      count: group.length,
      firstService: first,
      degraded: group.some((s) => s.status === "degraded" || s.status === "failed"),
    };
  });
}

function Rit({
  icon,
  label,
  on,
  badge,
  badgeTone,
  add,
}: {
  icon: IconId;
  label: string;
  on?: boolean;
  badge?: string;
  badgeTone?: "warn";
  add?: boolean;
}) {
  return (
    <span className={cn("rit", on && "on", add && "add")} role="img" aria-label={label}>
      <Icon id={icon} />
      {badge ? <span className={cn("rcnt", badgeTone)}>{badge}</span> : null}
    </span>
  );
}

export function Rail({ org, project, env, services, active }: RailProps) {
  const session = useSession();
  const alerting = services.some((s) => s.status === "degraded" || s.status === "failed");
  const search = { env: env ?? "production" };

  return (
    <nav className="rail" aria-label="Products">
      <Link to="/$org" params={{ org }}>
        <Rit icon="s-hex" label="Home" on={active.kind === "home"} />
      </Link>
      {/* Observe and Deploy are the only rail domains besides Home (ADR-017).
          Their surfaces land in Phase 2 — disabled with the stated reason, never hidden. */}
      <button
        type="button"
        disabled
        title="Observe — lands in Phase 2"
        className="cursor-not-allowed opacity-55"
      >
        <Rit icon="s-pulse" label="Observe" badge={alerting ? "1" : undefined} badgeTone="warn" />
      </button>
      <button
        type="button"
        disabled
        title="Deploy — lands in Phase 2"
        className="cursor-not-allowed opacity-55"
      >
        <Rit icon="s-deploy" label="Deploy" />
      </button>

      {services.length > 0 && <span className="my-1 h-px w-6 bg-hair" aria-hidden="true" />}

      {project
        ? productEntries(services).map((entry) => (
            <Link
              key={entry.product}
              to="/$org/$project/svc/$service"
              params={{ org, project, service: entry.firstService.name }}
              search={search}
            >
              <Rit
                icon={PRODUCT_ICON[entry.product]}
                label={PRODUCT_LABEL[entry.product]}
                on={active.kind === "product" && active.product === entry.product}
                badge={entry.count > 1 ? String(entry.count) : undefined}
              />
            </Link>
          ))
        : null}

      <button
        type="button"
        disabled
        title="Create — the creation canvas (C1) lands in Phase 2"
        className="cursor-not-allowed opacity-55"
      >
        <Rit icon="s-plus" label="Create service" add />
      </button>

      <span className="mt-auto" />
      <Link to="/$org/settings/audit" params={{ org }}>
        <Rit icon="s-gear" label="Settings" on={active.kind === "settings"} />
      </Link>
      <a href="https://docs.steloit.app" target="_blank" rel="noreferrer">
        <Rit icon="s-doc" label="Docs" />
      </a>
      <span className="uav" role="img" aria-label={session?.name ?? "Account"}>
        {session ? initialsOf(session.name) : "?"}
      </span>
    </nav>
  );
}
