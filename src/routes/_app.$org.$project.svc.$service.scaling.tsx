import { useMutation } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { MetricChart } from "@/design-system/chart";
import { EmptyState } from "@/design-system/empty-state";
import { Eyebrow } from "@/design-system/eyebrow";
import { Flabel, Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { toMarkers, toSeries, useMetrics } from "@/features/observe/hooks";
import { useServices } from "@/features/services/hooks";
import { updateServiceMutation } from "@/lib/api";
import { resolveEnvKey } from "@/lib/canon-env";
import { cn } from "@/lib/utils";

/**
 * D22 · Web/Worker · Scaling — a range, not a number. Apply patches through
 * updateService (scaling + the quarantined manual-pin override); the ladder,
 * cost rows and chart captions are frame-fixed from the D22 gallery frame.
 */

/** The ladder: floor 2 · now 3 · ceiling 6 · org policy caps at 6. */
const LADDER = [
  { label: "1" },
  { label: "2 · floor", on: true },
  { label: "3 · now", on: true },
  { label: "4" },
  { label: "5" },
  { label: "6 · ceiling", on: true },
  { label: "✕ 7+ · policy", policy: true },
];

/** A fresh instance's ladder — same default range, no "now" reading yet. */
const LADDER_FRESH = [
  { label: "1" },
  { label: "2 · floor", on: true },
  { label: "3" },
  { label: "4" },
  { label: "5" },
  { label: "6 · ceiling", on: true },
  { label: "✕ 7+ · policy", policy: true },
];

const SIGNALS = ["CPU · target 70%", "Requests / instance", "p95 latency"];

const COST_ROWS = [
  { label: "Floor · 2", value: "$28", note: "always on" },
  { label: "Now · 3", value: "$42", note: "this hour" },
  { label: "Ceiling · 6", value: "$84", note: "worst case" },
];

function ScalingPage() {
  const { project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));
  // Chart query is frame-fixed on the api requests series — the canon 24 h
  // telemetry that carries the ◆ #142 marker.
  const metrics = useMetrics(env, "service:api metric:requests");
  const update = useMutation(updateServiceMutation());
  const [signal, setSignal] = useState(SIGNALS[0]);
  const [pin, setPin] = useState("");
  const [reason, setReason] = useState("");

  const svc = services.data?.find((s) => s.name === service || s.id === service);
  if (!svc) return <main className="main" />;
  if (svc.product !== "web" && svc.product !== "worker") {
    return (
      <main className="main">
        <div className="pgpad">
          <h1 className="h1">Scaling</h1>
          <p className="hsub">
            Scaling (D22) is a web/worker surface — {svc.name} is {svc.product}.
          </p>
        </div>
      </main>
    );
  }

  // Instance counts, costs and the 24 h chart are the canon instance's
  // (web → api · worker → worker) frame-fixed telemetry — the policy chrome
  // (modes, ladder range, signals, pin) is config and applies to any instance.
  const canon = svc.name === (svc.product === "web" ? "api" : "worker");

  const pinSet = pin.trim() !== "" && pin.trim() !== "—";
  const applyDisabled = pinSet && reason.trim() === "";

  const apply = () => {
    // Canon-mode apply is echo-only: MSW has no PATCH /services/:service
    // handler, so the mutation fires for the API contract's sake and success
    // is toasted optimistically (same pattern as DB8's create).
    update.mutate({
      path: { service: svc.id },
      body: {
        scaling: { mode: "autoscale", floor: 2, ceiling: 6 },
        ...(pinSet && reason.trim()
          ? { override: { instances: Number(pin), reason: reason.trim() } }
          : {}),
      },
    });
    toast.success("Scaling updated — no restart · effective now · logged");
  };

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Scaling"
          sub={
            <span className="mono">
              {svc.name} · {env} · changes apply without a restart and land in the audit log
            </span>
          }
        >
          {canon ? (
            <Pill tone="ok">● autoscaling · 3 instances now</Pill>
          ) : (
            <Pill tone="ok">● autoscaling · floor 2 · ceiling 6</Pill>
          )}
        </Pghead>

        <div className="grid grid-cols-3 gap-3">
          <Card className="flex flex-col gap-1 p-3.5">
            <b className="text-12p5">Fixed</b>
            <span className="text-11 leading-relaxed text-ink3">
              Exactly N instances. Predictable cost, no reaction to traffic.
            </span>
          </Card>
          <Card className="flex flex-col gap-1 border-steel p-3.5">
            <b className="text-12p5">Autoscale</b>
            <span className="text-11 leading-relaxed text-ink3">
              A floor for reliability, a ceiling for cost, a signal in between.
            </span>
          </Card>
          <Card
            className="flex flex-col gap-1 p-3.5 opacity-55"
            title="For workers & previews — an HTTP service keeps a floor so first requests never cold-start."
          >
            <b className="text-12p5">Scale to zero</b>
          </Card>
        </div>

        <Card className="flex flex-col gap-3 p-4">
          <div className="chiprow">
            {(canon ? LADDER : LADDER_FRESH).map((step) => (
              <span
                key={step.label}
                className={cn(
                  "chip",
                  step.on && "on",
                  step.policy && "border-dashed border-err text-err",
                )}
              >
                {step.label}
              </span>
            ))}
          </div>
          <div className="flex items-center gap-3">
            <span className="text-11 text-ink3">
              drag the floor and ceiling handles — solid is paid always, dashed is paid only when
              traffic asks
            </span>
            <Pill tone="mut">org policy compute-ceiling: max 6 in production</Pill>
          </div>
          <div className="flex flex-col gap-1.5">
            <Flabel>Scale on</Flabel>
            <div className="chiprow">
              {SIGNALS.map((s) => (
                <button
                  key={s}
                  type="button"
                  className={cn("chip", signal === s && "on")}
                  aria-pressed={signal === s}
                  onClick={() => setSignal(s)}
                >
                  {s}
                </button>
              ))}
            </div>
          </div>
          <div className="text-12">Scale up after 1 min — Scale down after 5 min</div>
          <p className="text-11 text-ink3">
            up fast, down slow — flapping is treated as a bug, and the defaults encode that
          </p>
        </Card>

        <Card className="flex flex-col gap-2.5 p-4">
          <Eyebrow>What this range costs</Eyebrow>
          {canon ? (
            <>
              <div className="grid grid-cols-3 gap-3">
                {COST_ROWS.map((row) => (
                  <div key={row.label} className="flex items-baseline gap-2">
                    <span className="text-11p5 text-ink3">{row.label}</span>
                    <span className="mono text-13 font-semibold">{row.value}</span>
                    <span className="text-10p5 text-ink3">{row.note}</span>
                  </div>
                ))}
              </div>
              <p className="text-11p5 leading-relaxed text-ink2">
                Last 30 days on this range would have cost $47.20 — the range shows up in the
                project estimate as $28–84, never as a surprise.
              </p>
            </>
          ) : (
            <p className="text-11p5 leading-relaxed text-ink2">
              Floor, now and ceiling costs derive from this instance's shape and land here with the
              first billing hour — the range shows up in the project estimate as a floor–ceiling
              band, never as a surprise.
            </p>
          )}
          <p className="border-hair border-t pt-2.5 text-11 leading-relaxed text-ink3">
            Raising the ceiling never restarts anything; raising the size of instances does — that
            lives in Settings and states its blast radius there.
          </p>
        </Card>

        {!canon ? (
          <EmptyState
            compact
            icon="s-chart"
            title="No scaling history yet"
            meaning="the 24 h band draws as the service takes traffic · every scale event is annotated in Observe"
          />
        ) : (
          <Card className="flex flex-col gap-2.5 p-4">
            <div className="flex items-center gap-3">
              <Eyebrow>Last 24 h on this configuration</Eyebrow>
              <span className="mono text-10p5 text-ink3">
                ━ instances · ━ requests · band = floor↔ceiling
              </span>
              <span className="ml-auto text-10p5 text-ink3">
                every scale event is annotated in Observe
              </span>
            </div>
            <div className="relative">
              <MetricChart
                series={toSeries(metrics.data)}
                markers={toMarkers(metrics.data)}
                size="md"
              />
              <span className="mono absolute top-1 right-2 text-10 text-ink3">ceiling 6</span>
              <span className="mono absolute right-2 bottom-6 text-10 text-ink3">floor 2</span>
            </div>
            <div className="mono text-10p5 text-ink3">◆ deploy #142 — cpu, not traffic</div>
            <div className="mono text-10p5 text-ink2">
              2 → 3 with lunch traffic · 3 → 4 after #142 (the slow query burns CPU — fixing the
              index is cheaper than the extra instance)
            </div>
          </Card>
        )}

        <Card className="flex flex-col gap-2.5 border-warn p-4">
          <Eyebrow className="text-warn">
            Manual pin — suspends autoscale · banner on the service · auto-expires in 24 h
          </Eyebrow>
          <div className="flex items-center gap-2.5">
            <Inp
              className="mono w-20"
              placeholder="—"
              value={pin}
              onChange={(e) => setPin(e.target.value)}
              aria-label="Pinned instance count"
            />
            <Inp
              className="flex-1"
              placeholder="reason (required) — e.g. load test"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              aria-label="Pin reason"
            />
          </div>
          <div className="flex items-center gap-2.5">
            <Btn
              variant="p"
              disabled={applyDisabled}
              disabledReason="reason (required)"
              onClick={apply}
            >
              Apply changes
            </Btn>
            <span className="text-10p5 text-ink3">no restart · effective now · logged</span>
          </div>
        </Card>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/scaling")({
  component: ScalingPage,
});
