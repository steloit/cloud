import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { EmptyRow, EmptyState } from "@/design-system/empty-state";
import { Dot, Pill } from "@/design-system/pill";
import { SkeletonRows } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { AlertRuleDrawer } from "@/features/observe/alert-rule-drawer";
import { useAlertRules } from "@/features/observe/hooks";
import { ObserveChrome } from "@/features/observe/observe-chrome";
import type { AlertRule } from "@/lib/api";

/**
 * O5 · Observe — Alerts: anything findable is alertable. Rules come from the
 * canon fixtures via the API.
 *
 * NOTE: the O5 gallery frame shows 6 rules (dlq-depth, error-budget-burn,
 * missed-cron, cert-expiry) but the 19-canon fixtures define 4 with different
 * names — data follows fixtures (ADR-026); the divergence is flagged as a
 * finding, not resolved silently.
 */

/** Frame-fixed last-fired strings, keyed by canon rule name. */
const LAST_FIRED: Record<string, string> = {
  "p95-slo": "14:02 today",
  "dead-letters": "14:03 today",
  "slo-burn": "—",
  "cost-anomaly": "never",
};

const ROUTE_LABEL: Record<string, string> = {
  bell: "in-app",
  email: "email",
  webhook: "webhook",
};

function stateCell(rule: AlertRule) {
  if (rule.state === "ok") return <Pill tone="ok">ok</Pill>;
  const label =
    rule.state === "watching" && rule.burn_rate != null
      ? `watching · ${rule.burn_rate}×`
      : rule.state;
  return <Pill tone="warn">{label}</Pill>;
}

function RuleRow({ rule }: { rule: AlertRule }) {
  return (
    <tr
      style={
        rule.state === "firing"
          ? { background: "color-mix(in srgb, var(--warn) 6%, transparent)" }
          : undefined
      }
    >
      <td className="font-semibold">{rule.name}</td>
      <td className="mono">{rule.query}</td>
      <td className="mono">
        {rule.condition.op} {rule.condition.value} {rule.condition.unit ?? ""} · {rule.window}
      </td>
      <td>{stateCell(rule)}</td>
      <td>{LAST_FIRED[rule.name ?? ""] ?? "never"}</td>
      <td>{(rule.routes ?? []).map((r) => ROUTE_LABEL[r] ?? r).join(" + ")}</td>
    </tr>
  );
}

function AlertsPage() {
  const { project } = Route.useParams();
  const { env } = Route.useSearch();
  const rules = useAlertRules(project);
  const [tab, setTab] = useState("rules");
  const [drawerOpen, setDrawerOpen] = useState(false);

  const all = rules.data ?? [];
  const firing = all.filter((r) => r.state === "firing");
  const shown = tab === "firing" ? firing : all;
  // Tab counts derive from the live data the table filters — while the query
  // is unresolved they say "…", never a number the rows can contradict.
  // Silences has no endpoint; its 0 is frame-fixed (O5).
  const count = (n: number) => (rules.isSuccess ? n : "…");
  const tabs = [
    { id: "rules", label: `Rules · ${count(all.length)}` },
    { id: "firing", label: `Firing · ${count(firing.length)}` },
    { id: "history", label: "History" },
    { id: "silences", label: "Silences · 0" },
  ];
  const isEmpty = rules.isSuccess && all.length === 0;

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <ObserveChrome env={env} lens="Alerts" />

        <div className="flex items-center gap-3">
          <div className="tabrow flex-1">
            {tabs.map((t) => (
              <button
                key={t.id}
                type="button"
                className={tab === t.id ? "tab on" : "tab"}
                onClick={() => setTab(t.id)}
              >
                {t.label}
              </button>
            ))}
          </div>
          <Btn variant="p" onClick={() => setDrawerOpen(true)}>
            New alert rule
          </Btn>
        </div>

        {tab === "history" || tab === "silences" ? (
          <p className="text-11p5 text-ink3">
            {tab === "history" ? "History lands in Phase 3" : "Silences land in Phase 3"}
          </p>
        ) : rules.isError ? (
          <ApiFailureCard
            title="Alert rules didn't load"
            error={rules.error}
            requestLine={`GET /projects/${project}/alert-rules`}
            onRetry={() => rules.refetch()}
          />
        ) : isEmpty ? (
          <EmptyState
            compact
            icon="s-bell"
            title="No alert rules yet"
            meaning={
              <>
                anything findable is alertable — a rule is a query plus a condition, and it fires
                into the inbox (N2) through the same bell and email rules as everything else
              </>
            }
            cta={
              <Btn variant="p" onClick={() => setDrawerOpen(true)}>
                New alert rule
              </Btn>
            }
            cli="steloit alert create 'service:api metric:p95' --gt 800ms --window 5m"
          />
        ) : (
          <div className="tblwrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th>Rule</th>
                  <th>Query — the log/metric language, verbatim</th>
                  <th>Condition</th>
                  <th>State</th>
                  <th>Last fired</th>
                  <th>Routes to</th>
                </tr>
              </thead>
              <tbody>
                {rules.isPending ? (
                  <SkeletonRows cols={6} />
                ) : shown.length === 0 ? (
                  <EmptyRow cols={6}>
                    Nothing firing — all {all.length} rules are quiet right now
                  </EmptyRow>
                ) : (
                  shown.map((rule) => <RuleRow key={rule.id} rule={rule} />)
                )}
              </tbody>
            </table>
          </div>
        )}

        <div className="grid grid-cols-2 gap-3.5">
          <Card className="flex flex-col gap-2 p-4">
            <div className="text-12 font-semibold">New rule</div>
            <p className="text-11p5 text-ink3">
              opens the rule drawer — ⚑ buttons on Metrics &amp; Logs land there pre-filled
            </p>
            <div>
              <Btn variant="p" onClick={() => setDrawerOpen(true)}>
                New alert rule (U8)
              </Btn>
            </div>
          </Card>
          <Card className="flex items-start gap-2.5 p-4">
            <span className="pt-1">
              <Dot tone="ok" />
            </span>
            <p className="text-11p5 leading-relaxed text-ink2">
              Anything findable is alertable — rules fire into the inbox (N2); delivery is each
              person's (P6), escalation is org policy, and every silence is logged with actor +
              duration.
            </p>
          </Card>
        </div>

        {drawerOpen ? (
          <AlertRuleDrawer project={project} onClose={() => setDrawerOpen(false)} />
        ) : null}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/observe/alerts")({
  component: AlertsPage,
});
