import { Banner } from "@/design-system/banner";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Stlab } from "@/design-system/pill";
import { ConnectPanel, type VitalCell, VitalsStrip } from "./overview-zones";
import type { OverviewCtx } from "./overviews";

/**
 * X1 · AI Gateway overview — the shared Overview grammar over the capability
 * exemplar: vitals with cost last, the models table in the standard .tbl
 * contract, ConnectPanel for the bindings story. Rendered through the normal
 * service layout + SnavProduct like every other product.
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

const VITALS: VitalCell[] = [
  { label: "Requests · 24h", value: "128k", note: "+9%" },
  { label: "p95", value: "840 ms", note: "upstream-dominated" },
  { label: "Cache hit", value: "31%", note: "semantic cache · saves $" },
  { label: "Tokens · mtd", value: "18.2M", note: "in 12.1M · out 6.1M" },
  { label: "Cost", mono: true, value: "$34", note: "usage-priced" },
];

export function GatewayOverview({ nameOf: _nameOf }: OverviewCtx) {
  return (
    <>
      <Banner tone="default">
        X1 exemplar — the canon world's ecommerce runs 7 services totaling $208; adding the gateway
        to the data would break that arithmetic (finding), so this capability is chrome + this page,
        not a billed service.
      </Banner>
      <VitalsStrip cells={VITALS} />
      <div className="flex gap-3.5">
        <div className="flex flex-1 flex-col gap-3.5">
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
          <Card className="flex flex-col gap-2 p-4">
            <Eyebrow>Why it's a Service, not a new primitive</Eyebrow>
            <p className="text-[11.5px] leading-relaxed text-ink2">
              It satisfies the Service primitive (GOV-002). It differs only in <b>cardinality</b>:
              one endpoint, config-shaped.
            </p>
          </Card>
        </div>
        <ConnectPanel
          envPills={["AI_GATEWAY_URL — gw.acme-store.com", "AI_GATEWAY_KEY •••• · bnd_api_gw"]}
          cli={<Copybit>POST https://gw.acme-store.com/v1/chat</Copybit>}
          consumersEyebrow="Consumers · via bindings"
          consumers={[
            {
              icon: "s-globe",
              name: "api",
              pill: { tone: "mut", label: "invoke" },
              note: "rotated 12 d",
            },
            {
              icon: "s-worker",
              name: "worker",
              pill: { tone: "mut", label: "invoke" },
              note: "rotated 12 d",
            },
          ]}
          footer="Gateway keys are minted per consumer like every binding — no static secrets, rotated on policy."
        />
      </div>
    </>
  );
}
