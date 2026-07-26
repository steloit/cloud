import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Flabel, Inp } from "@/design-system/inp";
import { Drawer } from "@/design-system/overlay";
import { Stlab } from "@/design-system/pill";
import {
  type AlertCondition,
  backtestAlertRuleMutation,
  createAlertRuleMutation,
  errorMessage,
  listAlertRulesQueryKey,
} from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * U8 · New alert rule drawer (424px, over O5). The backtest is real: it fires
 * on open and (debounced) on every query/condition/window change, and the
 * firing line renders from the response — the canon incident (812 ms at
 * 14:02). Create posts through the alert-rules endpoint and invalidates O5's
 * rule list.
 */

const WINDOW_LABELS = ["1 min", "5 min", "15 min"] as const;
type WindowLabel = (typeof WINDOW_LABELS)[number];
const WINDOW_API: Record<WindowLabel, "1m" | "5m" | "15m"> = {
  "1 min": "1m",
  "5 min": "5m",
  "15 min": "15m",
};

type RouteKey = "bell" | "email" | "webhook";
const ROUTE_CHIPS: Array<{ key: RouteKey; label: string }> = [
  { key: "bell", label: "in-app bell" },
  { key: "email", label: "email" },
  { key: "webhook", label: "webhook" },
];

/** "> 400 ms" → the API's structured condition. */
function parseCondition(text: string): AlertCondition {
  const m = text.match(/([<>]=?)\s*([\d.]+)\s*(.*)/);
  return {
    op: (m?.[1] ?? ">") as AlertCondition["op"],
    value: Number(m?.[2] ?? 400),
    unit: m?.[3]?.trim() || "ms",
  };
}

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

/** "2026-07-02T14:02:00+05:30" → "Jul 2 14:02" — string-parsed so canon IST survives any local TZ. */
function fmtFiredAt(at: string | undefined): string {
  const m = (at ?? "").match(/-(\d{2})-(\d{2})T(\d{2}):(\d{2})/);
  if (!m) return "";
  return `${MONTHS[Number(m[1]) - 1]} ${Number(m[2])} ${m[3]}:${m[4]}`;
}

export function AlertRuleDrawer({ project, onClose }: { project: string; onClose: () => void }) {
  const [query, setQuery] = useState("http.p95{service=api,env=production}");
  const [conditionText, setConditionText] = useState("> 400 ms");
  const [windowLabel, setWindowLabel] = useState<WindowLabel>("5 min");
  const [routes, setRoutes] = useState<RouteKey[]>(["bell", "email"]);
  const [webhookUrl, setWebhookUrl] = useState("");

  const queryClient = useQueryClient();
  const backtest = useMutation(backtestAlertRuleMutation());
  const create = useMutation({
    ...createAlertRuleMutation(),
    onSuccess: () => {
      toast.success("Alert rule created — rule 7 · production");
      queryClient.invalidateQueries({ queryKey: listAlertRulesQueryKey({ path: { project } }) });
      onClose();
    },
    onError: (err) => toast.error(errorMessage(err)),
  });

  const { mutate: runBacktest } = backtest;
  useEffect(() => {
    // Fires on open (initial run) and debounced on query/condition/window edits.
    const t = setTimeout(() => {
      runBacktest({
        body: {
          query,
          condition: parseCondition(conditionText),
          window: WINDOW_API[windowLabel],
          days: 7,
        },
      });
    }, 350);
    return () => clearTimeout(t);
  }, [runBacktest, query, conditionText, windowLabel]);

  const firings = backtest.data?.firings ?? [];
  const firing = firings[0];

  const submit = () => {
    create.mutate({
      path: { project },
      body: {
        // name auto from query — the metric path, dash-cased.
        name: query.split("{")[0]?.replace(/\./g, "-") ?? "new-rule",
        query,
        condition: parseCondition(conditionText),
        window: WINDOW_API[windowLabel],
        routes,
      },
    });
  };

  return (
    <Drawer
      title="New alert rule"
      sub="ecommerce / production · fires through Observe"
      onClose={onClose}
      footer={
        <>
          <span className="mono text-10p5 text-ink3">Rule 7 · production</span>
          <span className="flex-1" />
          <Btn variant="s" onClick={onClose}>
            Cancel
          </Btn>
          <Btn variant="p" onClick={submit}>
            Create rule
          </Btn>
        </>
      }
    >
      <div>
        <Flabel htmlFor="rule-query">Query · pre-filled from Metrics ⚑</Flabel>
        <Inp
          id="rule-query"
          className="mono"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>

      <div className="flex items-end gap-2.5">
        <div className="flex-1">
          <Flabel htmlFor="rule-condition">Condition</Flabel>
          <Inp
            id="rule-condition"
            className="mono"
            value={conditionText}
            onChange={(e) => setConditionText(e.target.value)}
          />
        </div>
        <div>
          <Flabel htmlFor="rule-window">Window</Flabel>
          <select
            id="rule-window"
            className="inp"
            value={windowLabel}
            onChange={(e) => setWindowLabel(e.target.value as WindowLabel)}
          >
            {WINDOW_LABELS.map((w) => (
              <option key={w} value={w}>
                {w}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="flex flex-col gap-1.5">
        <Flabel>Route to</Flabel>
        <div className="chiprow">
          {ROUTE_CHIPS.map((r) => (
            <button
              key={r.key}
              type="button"
              className={cn("chip", routes.includes(r.key) && "on")}
              aria-pressed={routes.includes(r.key)}
              onClick={() =>
                setRoutes((prev) =>
                  prev.includes(r.key) ? prev.filter((k) => k !== r.key) : [...prev, r.key],
                )
              }
            >
              {r.label}
            </button>
          ))}
        </div>
        {routes.includes("webhook") ? (
          // Webhook config expands INLINE with the chip — drawer-over-drawer
          // is the wrong recipe, so this never opens a second overlay; it
          // collapses when the chip is deselected.
          <Card className="flex flex-col gap-2.5 p-3.5">
            <div>
              <Flabel htmlFor="webhook-url">Webhook URL</Flabel>
              <Inp
                id="webhook-url"
                className="mono"
                placeholder="https://"
                value={webhookUrl}
                onChange={(e) => setWebhookUrl(e.target.value)}
              />
            </div>
            <div className="flex items-center gap-2.5">
              <span className="text-11 text-ink2">Signing secret</span>
              <span className="chip mono">whsec_••••••••••••</span>
              <span className="text-10p5 text-ink3">revealed once at rule create</span>
              <span className="flex-1" />
              <Btn
                variant="s"
                disabled
                disabledReason="Webhook delivery lands with the notifications endpoints (finding)"
              >
                Send test delivery
              </Btn>
            </div>
          </Card>
        ) : null}
      </div>

      <div className="flex flex-col gap-1.5">
        <Flabel aside={<Stlab tone="ok">ran just now</Stlab>}>Backtest — last 7 days</Flabel>
        <div className="logwell">
          {firing ? (
            <>
              <div>
                would have fired {firings.length}× — {fmtFiredAt(firing.at)} · p95 {firing.value} ms
                (the checkout incident)
              </div>
              {/* Second line frame-fixed — the backtest response carries firings only. */}
              <div className="t">quiet otherwise · median p95 318 ms</div>
            </>
          ) : backtest.isPending || backtest.isIdle ? (
            <div className="t">running backtest…</div>
          ) : backtest.isError ? (
            // A failed backtest must never read as "no firings" — the quiet
            // result and the missing result are different truths.
            <div className="flex items-center gap-2">
              <span className="text-err">backtest didn't run — check the query</span>
              <Btn
                variant="gh"
                className="h-5 px-1.5 text-10"
                onClick={() =>
                  runBacktest({
                    body: {
                      query,
                      condition: parseCondition(conditionText),
                      window: WINDOW_API[windowLabel],
                      days: 7,
                    },
                  })
                }
              >
                Retry
              </Btn>
            </div>
          ) : (
            <div className="t">no firings in the last 7 days</div>
          )}
        </div>
        <p className="text-10p5 text-ink3">
          A backtest is how you tune a threshold without a week of false pages.
        </p>
      </div>

      <Card className="p-3.5 text-11 leading-relaxed text-ink2">
        Fires through Observe like everything else — same bell, same email rules, your quiet hours
        (P6) respected for routing, never for recording.
      </Card>
    </Drawer>
  );
}
