import { Pghead } from "@/app/shell/pghead";
import { Nfoot, NitDisabled, NitLink, Nsec, Snav } from "@/app/shell/snav";
import { Banner } from "@/design-system/banner";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Icon } from "@/design-system/icon";
import { Metric } from "@/design-system/metric";
import { Pill, Stlab } from "@/design-system/pill";

/**
 * X1 · AI Gateway — a capability-shaped Service proving the rail generalizes:
 * no ×n badge, no instance switcher; the sidebar holds routes/models, not
 * instances. Finding: no ai-gateway service exists in 19-canon — adding one
 * would break the $208 arithmetic invariant (7 services = $208), so this
 * exemplar is URL-reachable (svc/gateway) but absent from the rail/topology.
 */

const MODELS = [
  {
    alias: "fast",
    upstream: "claude-haiku",
    weight: "70%",
    p95: "410ms",
    price: "$0.25",
    state: "healthy" as const,
  },
  {
    alias: "smart",
    upstream: "claude-sonnet",
    weight: "25%",
    p95: "920ms",
    price: "$3.00",
    state: "healthy" as const,
  },
  {
    alias: "vision",
    upstream: "gpt-4o",
    weight: "5%",
    p95: "1.2s",
    price: "$5.00",
    state: "healthy" as const,
  },
  {
    alias: "fallback",
    upstream: "llama-local",
    weight: "on error",
    p95: "300ms",
    price: "$0.04",
    state: "standby" as const,
  },
];

export function GatewayPage({ org, project }: { org: string; project: string }) {
  return (
    <>
      <Snav>
        <div className="snhead">
          <span className="glyph">
            <Icon id="s-ai" />
          </span>
          <div>
            <div className="t">AI Gateway</div>
            <div className="u">capability · one endpoint</div>
          </div>
        </div>
        <NitLink
          to="/$org/$project/svc/$service"
          params={{ org, project, service: "gateway" }}
          search={{ env: "production" }}
          icon="s-eye"
          label="Overview"
          on
        />
        <NitDisabled
          icon="s-ai"
          label="Models"
          count="4"
          reason="Model management lands with a canon gateway service"
        />
        <NitDisabled
          icon="s-branch"
          label="Routes"
          count="3"
          reason="Route weights land with a canon gateway service"
        />
        <NitDisabled
          icon="s-card"
          label="Usage & cost"
          reason="Usage lands with a canon gateway service"
        />
        <NitDisabled
          icon="s-shield"
          label="Policies"
          reason="Gateway policies land with a canon gateway service"
        />
        <NitDisabled
          icon="s-key"
          label="Keys & bindings"
          reason="Bindings land with a canon gateway service"
        />
        <NitDisabled
          icon="s-gear"
          label="Settings"
          reason="Settings land with a canon gateway service"
        />
        <Nsec> </Nsec>
        <Nfoot>
          <div className="flex justify-between px-1.5">
            <span>AI Gateway</span>
            <span className="mono">$34/mo</span>
          </div>
          <div className="px-1.5 pt-1">usage-priced · no instances to scale</div>
        </Nfoot>
      </Snav>
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Banner tone="default">
            X1 exemplar — the canon world's ecommerce runs 7 services totaling $208; adding the
            gateway would break that arithmetic (finding), so this page is URL-reachable but off the
            rail.
          </Banner>
          <Pghead
            title={
              <span className="flex items-center gap-2.5">
                AI Gateway <Pill tone="st">capability</Pill>
              </span>
            }
            sub="One endpoint in ecommerce / production · 4 models behind 3 routes · a service with no fleet — config, not instances"
          >
            <Copybit>POST https://gw.acme-store.com/v1/chat</Copybit>
          </Pghead>
          <div className="grid grid-cols-5 gap-3">
            <Metric label="Requests · 24h" value="128k" note="+9%" />
            <Metric label="p95" value="840 ms" note="upstream-dominated" />
            <Metric label="Cache hit" value="31%" note="semantic cache · saves $" />
            <Metric label="Tokens · mtd" value="18.2M" note="in 12.1M · out 6.1M" />
            <Metric label="Cost" mono value="$34" note="usage-priced" />
          </div>
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
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="text-[11.5px] text-ink3">
            Scaling is <b>weights &amp; fallbacks</b>, not instance counts — so no ×n rail badge and
            no instance switcher (SW3).
          </p>
          <div className="grid grid-cols-2 gap-3.5">
            <Card className="flex flex-col gap-2 p-4">
              <Eyebrow>Connect</Eyebrow>
              <div className="mono text-[11px] text-ink2">AI_GATEWAY_URL gw.acme-store.com</div>
              <div className="mono text-[11px] text-ink2">AI_GATEWAY_KEY •••• bnd_api_gw</div>
              <p className="text-[11px] text-ink3">
                Consumed by <b>api</b> and <b>worker</b> via bindings.
              </p>
            </Card>
            <Card className="flex flex-col gap-2 p-4">
              <Eyebrow>Why it's a Service, not a new primitive</Eyebrow>
              <p className="text-[11.5px] leading-relaxed text-ink2">
                It satisfies the Service primitive (GOV-002). It differs only in <b>cardinality</b>:
                one endpoint, config-shaped.
              </p>
            </Card>
          </div>
        </div>
      </main>
    </>
  );
}
