import { Link } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Icon } from "@/design-system/icon";
import { Metric } from "@/design-system/metric";
import { Pill, Stlab } from "@/design-system/pill";
import { usePolicies } from "@/features/settings/hooks";
import type { Service } from "@/lib/api";

/**
 * X1 sub-tabs — the gallery frames only the gateway's Overview; these pages
 * reuse the console's existing grammars (D-series tables, B2 meters, G4
 * policy rows, D11 bindings) with content derived from X1's verbatim canon.
 * Where a detail isn't specced it follows fixtures' own "$representative"
 * convention and says so (finding: X1 sub-tab frames don't exist).
 */

interface TabProps {
  svc: Service;
  org: string;
  project: string;
  env: string;
}

const MODELS = [
  {
    alias: "fast",
    upstream: "claude-haiku",
    weight: "70%",
    p95: "410ms",
    price: "$0.25",
    state: "healthy" as const,
    note: "default for chat traffic",
  },
  {
    alias: "smart",
    upstream: "claude-sonnet",
    weight: "25%",
    p95: "920ms",
    price: "$3.00",
    state: "healthy" as const,
    note: "complex prompts route up",
  },
  {
    alias: "vision",
    upstream: "gpt-4o",
    weight: "5%",
    p95: "1.2s",
    price: "$5.00",
    state: "healthy" as const,
    note: "image inputs only",
  },
  {
    alias: "fallback",
    upstream: "llama-local",
    weight: "on error",
    p95: "300ms",
    price: "$0.04",
    state: "standby" as const,
    note: "serves when upstreams fail",
  },
];

export function GatewayModelsTab(_props: TabProps) {
  return (
    <>
      <Pghead
        title="AI Gateway · Models"
        sub="Four upstreams behind stable aliases — consumers call the alias; you re-point it without a deploy"
      >
        <Btn
          variant="p"
          disabled
          disabledReason="Model management needs a canon gateway service (X1 finding)"
        >
          <Icon id="s-plus" />
          Add model
        </Btn>
      </Pghead>
      <div className="tblwrap">
        <table className="tbl">
          <thead>
            <tr>
              <th>Alias</th>
              <th>Upstream</th>
              <th>Route weight</th>
              <th>p95</th>
              <th>$/1k tok</th>
              <th>State</th>
              <th>Why it's here</th>
            </tr>
          </thead>
          <tbody>
            {MODELS.map((m) => (
              <tr key={m.alias}>
                <td className="mono font-semibold">{m.alias}</td>
                <td className="mono">{m.upstream}</td>
                <td>{m.weight}</td>
                <td className="mono">{m.p95}</td>
                <td className="mono">{m.price}</td>
                <td>
                  <Stlab tone={m.state === "healthy" ? "ok" : "sus"}>{m.state}</Stlab>
                </td>
                <td className="text-ink3">{m.note}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="text-[11px] text-ink3">
        Aliases are the contract: swapping an upstream behind <span className="mono">fast</span>{" "}
        never touches a consumer. "Why it's here" notes are representative — X1 specs the model
        table without per-row rationale (finding).
      </p>
    </>
  );
}

/** Route names are representative — X1 specs "3 routes" without naming them (finding). */
const ROUTES = [
  {
    path: "/v1/chat",
    models: "fast 70% · smart 25% · vision 5%",
    fallback: "llama-local on error",
    traffic: "119k/24h",
  },
  { path: "/v1/embed", models: "fast 100%", fallback: "—", traffic: "8.2k/24h" },
  { path: "/v1/vision", models: "vision 100%", fallback: "smart on error", traffic: "0.8k/24h" },
];

export function GatewayRoutesTab(_props: TabProps) {
  return (
    <>
      <Pghead
        title="AI Gateway · Routes"
        sub="Weighted routing over the model aliases — scaling is weights & fallbacks, not instance counts"
      >
        <Btn
          variant="p"
          disabled
          disabledReason="Route management needs a canon gateway service (X1 finding)"
        >
          <Icon id="s-plus" />
          New route
        </Btn>
      </Pghead>
      <div className="tblwrap">
        <table className="tbl">
          <thead>
            <tr>
              <th>Route</th>
              <th>Weights</th>
              <th>Fallback</th>
              <th>Traffic · 24h</th>
            </tr>
          </thead>
          <tbody>
            {ROUTES.map((r) => (
              <tr key={r.path}>
                <td className="mono font-semibold">{r.path}</td>
                <td className="mono">{r.models}</td>
                <td className="mono">{r.fallback}</td>
                <td className="mono">{r.traffic}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="text-[11px] text-ink3">
        Route names and splits are representative — X1 says "4 models behind 3 routes" without
        naming them (finding). Weight changes apply without a restart and land in the audit log.
      </p>
    </>
  );
}

export function GatewayUsageTab(_props: TabProps) {
  return (
    <>
      <Pghead
        title="AI Gateway · Usage & cost"
        sub="Tokens are infrastructure — metered like any resource, not a plan quota (B7). The ledger stays in Billing; this watches it."
      >
        <Link to="/$org/billing/usage" params={{ org: _props.org }}>
          <Btn variant="s">Open Billing →</Btn>
        </Link>
      </Pghead>
      <div className="grid grid-cols-5 gap-3">
        <Metric label="Requests · 24h" value="128k" note="+9%" />
        <Metric label="Tokens in · mtd" value="12.1M" note="prompt side" />
        <Metric label="Tokens out · mtd" value="6.1M" note="completion side" />
        <Metric label="Cache hit" value="31%" note="semantic cache · saves $" />
        <Metric label="Cost · mtd" mono value="$34" note="usage-priced · no floor" />
      </div>
      <div className="tblwrap">
        <table className="tbl">
          <thead>
            <tr>
              <th>Meter</th>
              <th>Used · mtd</th>
              <th>Unit price</th>
              <th>MTD</th>
            </tr>
          </thead>
          <tbody>
            {MODELS.map((m) => (
              <tr key={m.alias}>
                <td>
                  <b>{m.alias}</b> <span className="text-ink3">· {m.upstream}</span>
                </td>
                <td className="mono">
                  {m.alias === "fast"
                    ? "13.9M tok"
                    : m.alias === "smart"
                      ? "3.6M tok"
                      : m.alias === "vision"
                        ? "0.6M tok"
                        : "0.1M tok"}
                </td>
                <td className="mono">{m.price} / 1k tok</td>
                <td className="mono">
                  {m.alias === "fast"
                    ? "$3.48"
                    : m.alias === "smart"
                      ? "$10.80"
                      : m.alias === "vision"
                        ? "$3.00"
                        : "$0.04"}
                </td>
              </tr>
            ))}
            <tr>
              <td colSpan={3}>
                <b>Semantic cache credit</b>{" "}
                <span className="text-ink3">· 31% of requests served without an upstream call</span>
              </td>
              <td className="mono text-ok">−$7.80</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p className="text-[11px] text-ink3">
        Per-model splits are representative reconciliations of X1's totals (18.2M tokens · $34 mtd)
        — the frame specs totals only (finding). Unit prices are printed here, not discovered on the
        invoice.
      </p>
    </>
  );
}

export function GatewayPoliciesTab({ org, project }: TabProps) {
  const policies = usePolicies(org);
  const relevant = (policies.data ?? []).filter((p) =>
    ["ai-assistant", "credential-rotation", "allowed-regions"].includes(p.key ?? ""),
  );
  return (
    <>
      <Pghead
        title="AI Gateway · Policies"
        sub="What applies to this capability — the same org rulebook, no gateway-specific policy engine"
      >
        <Link to="/$org/settings/policies" params={{ org }}>
          <Btn variant="s">Org rulebook (G7) →</Btn>
        </Link>
      </Pghead>
      <div className="tblwrap">
        <table className="tbl">
          <thead>
            <tr>
              <th>Policy</th>
              <th>Rule</th>
              <th>Scope</th>
              <th>Enforcement</th>
            </tr>
          </thead>
          <tbody>
            {relevant.map((p) => (
              <tr key={p.id}>
                <td>
                  <b className="mono">{p.key}</b>
                </td>
                <td className="text-ink2">{p.description}</td>
                <td>
                  <Pill tone="mut">inherited · org</Pill>
                </td>
                <td>
                  <Pill tone={p.enforcement === "enabled" ? "ok" : "st"}>{p.enforcement}</Pill>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <Card className="p-3.5 text-[11.5px] leading-relaxed text-ink3">
        Gateway <b>tokens</b> are infrastructure and meter regardless of the{" "}
        <span className="mono">ai-assistant</span> policy; that switch governs the Assistant's
        surfaces (AI3), not this service. Rows are live from the org rulebook — this page adds no
        gateway-only rules.
      </Card>
      <span className="hidden">{project}</span>
    </>
  );
}

export function GatewayKeysTab({ project, env }: TabProps) {
  return (
    <>
      <Pghead
        title="AI Gateway · Keys & bindings"
        sub="gateway · 2 consumers · credentials minted per consumer, least-privilege, rotated"
      >
        <Btn
          variant="s"
          disabled
          disabledReason="Bindings need a canon gateway service (X1 finding)"
        >
          Create binding (U2)
        </Btn>
      </Pghead>
      <div className="tblwrap">
        <table className="tbl">
          <thead>
            <tr>
              <th>Binding</th>
              <th>Consumer</th>
              <th>Role</th>
              <th>Rotated</th>
              <th>Last used</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {[
              { id: "bnd_api_gw", name: "api", icon: "s-globe" as const, used: "now · 41 req/min" },
              { id: "bnd_worker_gw", name: "worker", icon: "s-worker" as const, used: "2 m ago" },
            ].map((b) => (
              <tr key={b.id}>
                <td className="mono">{b.id}</td>
                <td>
                  <span className="flex items-center gap-2">
                    <Icon id={b.icon} className="h-3.5 w-3.5 text-ink3" />
                    <b>{b.name}</b>
                  </span>
                </td>
                <td>
                  <Pill tone="st">invoke</Pill>
                </td>
                <td>12 d ago</td>
                <td className="text-ink3">{b.used}</td>
                <td>
                  <span className="flex gap-2">
                    <Btn
                      variant="gh"
                      className="h-6 px-2 text-[10.5px]"
                      disabled
                      disabledReason="No rotate endpoint in the spec — spec change first (finding)"
                    >
                      Rotate
                    </Btn>
                    <Btn
                      variant="gh"
                      className="h-6 px-2 text-[10.5px] text-err"
                      disabled
                      disabledReason="Bindings need a canon gateway service (X1 finding)"
                    >
                      Revoke
                    </Btn>
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="flex items-center gap-2.5">
        <span className="bindpill">
          <Icon id="s-bind" />
          AI_GATEWAY_URL — gw.acme-store.com
        </span>
        <span className="bindpill">
          <Icon id="s-key" />
          AI_GATEWAY_KEY •••• · minted per consumer
        </span>
        <Copybit>POST https://gw.acme-store.com/v1/chat</Copybit>
      </div>
      <p className="text-[11px] text-ink3">
        Same contract as every binding (D11): consumers never see a key; the binding injects and
        refreshes it. Rotation: 90 d automatic (org policy{" "}
        <span className="mono">credential-rotation</span>).
      </p>
      <span className="hidden">
        {project}
        {env}
      </span>
    </>
  );
}

export function GatewaySettingsTab(_props: TabProps) {
  return (
    <>
      <Pghead
        title="AI Gateway · Settings"
        sub="gateway · production · service-scoped — this page never leaves the service (§4.6)"
      />
      <Card className="flex flex-col gap-3 p-4">
        <Eyebrow>Endpoint & cache</Eyebrow>
        <div className="grid grid-cols-3 gap-4 text-[12px]">
          <div>
            <div className="flabel">Endpoint</div>
            <div className="inp mono">gw.acme-store.com</div>
          </div>
          <div>
            <div className="flabel">Semantic cache</div>
            <div className="inp">on · 31% hit</div>
          </div>
          <div>
            <div className="flabel">Fallback</div>
            <div className="inp mono">llama-local · on error</div>
          </div>
        </div>
      </Card>
      <Card className="flex flex-col gap-2.5 border-err/45 p-4">
        <Eyebrow className="text-err">Danger zone</Eyebrow>
        <div className="flex items-center gap-3">
          <Btn
            variant="dgr"
            disabled
            disabledReason="X1 is an exemplar — there is no canon gateway service to delete (finding)"
          >
            Delete gateway…
          </Btn>
          <Pill tone="err">exemplar — not a billed service</Pill>
        </div>
        <p className="text-[11px] text-ink3">
          When a canon gateway exists, deletion follows the U6 grammar: dependents named, typed
          confirm, final state kept.
        </p>
      </Card>
    </>
  );
}
