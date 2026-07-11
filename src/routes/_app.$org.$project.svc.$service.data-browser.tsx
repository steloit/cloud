import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { EmptyState } from "@/design-system/empty-state";
import { Glyph } from "@/design-system/glyph";
import { Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { useServices } from "@/features/services/hooks";
import { resolveEnvKey } from "@/lib/canon-env";
import { cn } from "@/lib/utils";

/**
 * D3 · Valkey Data Browser — 08-api has no key-browse endpoint although D3
 * requires one (finding: spec change needed before this page can go live);
 * the keyspace below is frame-fixed canon from the D3 frame.
 */

const TYPE_CHIPS = ["all", "string", "hash", "list", "set"] as const;
type TypeChip = (typeof TYPE_CHIPS)[number];

interface KeyRow {
  key: string;
  type: "string" | "hash" | "list" | "set";
  ttl: string;
  selected?: boolean;
}

const KEYS: KeyRow[] = [
  { key: "session:u_8241", type: "hash", ttl: "ttl 24m", selected: true },
  { key: "session:u_1177", type: "hash", ttl: "ttl 8m" },
  { key: "session:u_10422", type: "hash", ttl: "ttl 41m" },
  { key: "cart:u_8241", type: "hash", ttl: "ttl 2h" },
  { key: "rate:api:10.2.4.1", type: "string", ttl: "ttl 58s" },
  { key: "feed:home", type: "list", ttl: "no ttl" },
];

const FIELDS: Array<[string, string]> = [
  ["user_id", "8241"],
  ["cart_items", '["gc_77b2","sku_1204","sku_887"]'],
  ["subtotal", "4180.00"],
  ["currency", "INR"],
  ["last_seen", "2026-07-06T14:18:02Z"],
];

// Built by hand so 4180.00 keeps its cents — JSON.stringify would drop them.
const DETAIL_JSON = `{
  "user_id": 8241,
  "cart_items": ["gc_77b2", "sku_1204", "sku_887"],
  "subtotal": 4180.00,
  "currency": "INR",
  "last_seen": "2026-07-06T14:18:02Z"
}`;

function DataBrowserPage() {
  const { project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));
  const [typeChip, setTypeChip] = useState<TypeChip>("all");

  const svc = services.data?.find((s) => s.name === service || s.id === service);
  if (!svc) return <main className="main" />;
  if (svc.product !== "valkey") {
    return (
      <main className="main">
        <div className="pgpad">
          <Card className="p-4 text-12 text-ink2">
            Data Browser is a Valkey surface — {svc.name} is a {svc.product} service.
          </Card>
        </div>
      </main>
    );
  }

  // The keyspace above is `cache`'s frame-fixed canon — any other valkey
  // instance starts honest: an empty keyspace until consumers write.
  if (svc.name !== "cache") {
    return (
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            before={<Glyph id="s-chip" />}
            title="Data Browser"
            sub="cache-mode · LRU eviction · read-only by default"
          />
          <EmptyState
            icon="s-chip"
            title="Keyspace is empty"
            meaning="keys appear as consumers write · TTLs and memory show per key"
            cli={`steloit valkey cli ${svc.name}`}
          />
        </div>
      </main>
    );
  }

  const shown = KEYS.filter((k) => typeChip === "all" || k.type === typeChip);

  const copyJson = async () => {
    await navigator.clipboard.writeText(DETAIL_JSON);
    toast.success("JSON copied");
  };

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          before={<Glyph id="s-chip" />}
          title="Data Browser"
          sub="cache-mode · LRU eviction · read-only by default"
        />

        <div className="flex items-center gap-3">
          <div className="chiprow">
            {TYPE_CHIPS.map((t) => (
              <button
                key={t}
                type="button"
                className={cn("chip", typeChip === t && "on")}
                onClick={() => setTypeChip(t)}
              >
                {t}
              </button>
            ))}
          </div>
          <div className="w-[220px]">
            <Inp
              className="mono"
              value="session:*"
              readOnly
              aria-label="Search keys"
              title="No key-browse endpoint in the spec (finding) — the pattern is frame-fixed canon"
            />
          </div>
          <Pill tone="mut">1,842 keys match</Pill>
        </div>

        <div className="flex gap-3.5">
          <Card className="w-[340px] shrink-0 self-start">
            {shown.map((k) => (
              <div
                key={k.key}
                className={cn(
                  "flex items-center gap-2.5 border-hair border-b px-3.5 py-2.5 last:border-b-0",
                  k.selected && "bg-steel-tint",
                )}
              >
                <span className="mono flex-1 truncate text-11p5">{k.key}</span>
                <Pill tone="mut">{k.type}</Pill>
                <span className="mono text-10p5 text-ink3">{k.ttl}</span>
              </div>
            ))}
          </Card>

          <Card className="flex flex-1 flex-col gap-3 self-start p-4">
            <div className="flex items-center gap-2.5">
              <b className="mono text-13">session:u_8241</b>
              <Pill tone="mut">hash</Pill>
              <span className="mono text-11 text-ink3">TTL 24m 12s</span>
              <span className="mono text-11 text-ink3">2.1 KB</span>
            </div>
            <div className="tblwrap">
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Field</th>
                    <th>Value</th>
                  </tr>
                </thead>
                <tbody>
                  {FIELDS.map(([field, value]) => (
                    <tr key={field}>
                      <td className="mono">{field}</td>
                      <td className="mono">{value}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="flex items-center gap-2.5 border-hair border-t pt-3">
              <Btn variant="s" onClick={copyJson}>
                Copy JSON
              </Btn>
              <Btn
                variant="s"
                disabled
                disabledReason="No key-browse endpoint in the spec (finding)"
              >
                Edit TTL
              </Btn>
              <Btn
                variant="dgr"
                disabled
                disabledReason="No key-browse endpoint in the spec (finding)"
              >
                Delete key
              </Btn>
            </div>
            <p className="text-10p5 text-ink3">
              Deletes and TTL edits require read-write · logged to the audit stream
            </p>
          </Card>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/data-browser")({
  component: DataBrowserPage,
});
