import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { EmptyState } from "@/design-system/empty-state";
import { Glyph } from "@/design-system/glyph";
import { Pill } from "@/design-system/pill";
import { SkeletonRows } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { useServices } from "@/features/services/hooks";
import { LifecycleDrawer } from "@/features/services/lifecycle-drawer";
import { type LifecycleRule, listLifecycleRulesOptions } from "@/lib/api";
import type { CanonProduct } from "@/lib/api/legacy";
import { resolveEnvKey } from "@/lib/canon-env";

/**
 * D15 · Lifecycle rules — the table rides the real lifecycle-rules endpoint;
 * last-run and affected counts are frame-fixed display (the schema carries
 * no run history — finding). The D15 frame shows an `uploads/raw/ · 30 d`
 * row, but the canon world data has `originals/ · tier_cold · 90 d` —
 * fixtures-style world wins over the frame.
 */

// Keyed on the canon fixture ids (not prefixes) so a non-canon rule that reuses
// a canon prefix never inherits canon run history — it degrades to "not yet
// run" / "—", and that degradation is intentional.
const DISPLAY: Record<string, { last: string; affected: string }> = {
  rule_tmp_expire: { last: "today · 03:00", affected: "2,041 objects" },
  rule_originals_cold: { last: "today · 03:00", affected: "11,384 objects" },
};

function actionLabel(action: LifecycleRule["action"]): string {
  return action === "expire" ? "delete" : "→ cold tier";
}

function LifecyclePage() {
  const { project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));
  const [drawerOpen, setDrawerOpen] = useState(false);

  const svc = services.data?.find((s) => s.name === service || s.id === service);
  const rules = useQuery({
    ...listLifecycleRulesOptions({ path: { service: svc?.id ?? "" } }),
    select: (r) => r.data ?? [],
    enabled: Boolean(svc),
  });

  if (!svc) return <main className="main" />;
  if ((svc.product as CanonProduct) !== "storage") {
    return (
      <main className="main">
        <div className="pgpad">
          <Card className="p-4 text-12 text-ink2">
            Lifecycle rules are an Object Storage surface — {svc.name} is a {svc.product} service.
          </Card>
        </div>
      </main>
    );
  }

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        {/* Finding: the frame shows the bare "Lifecycle rules" title — the design-system
            "Area · Thing" h1 grammar wins per the audit's P1 ruling. */}
        <Pghead
          before={<Glyph id="s-bucket" />}
          title="Object Storage · Lifecycle rules"
          sub={
            <span className="mono">
              {svc.name} · rules run daily at 03:00 · every action lands in the audit log
            </span>
          }
        >
          <Btn variant="s" onClick={() => setDrawerOpen(true)}>
            Add rule (U3)
          </Btn>
        </Pghead>

        {/* Four-state grammar (16-qa): pending → skeleton, error → failure
            card, empty → EmptyState, else table. */}
        {rules.isError ? (
          <ApiFailureCard
            title="Lifecycle rules didn't load"
            error={rules.error}
            requestLine={`GET /services/${svc.id}/lifecycle-rules`}
            onRetry={() => rules.refetch()}
          />
        ) : !rules.isPending && (rules.data ?? []).length === 0 ? (
          <EmptyState
            compact
            icon="s-bucket"
            title="No lifecycle rules yet"
            meaning={
              <>
                rules tier or expire objects automatically — a dry-run shows exactly what a rule
                would touch before it applies
              </>
            }
            cta={
              <Btn variant="s" onClick={() => setDrawerOpen(true)}>
                Add rule (U3)
              </Btn>
            }
            cli="steloit storage lifecycle add tmp/ --expire 7d --dry-run"
          />
        ) : (
          <Card>
            <div className="tblwrap">
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Prefix</th>
                    <th>Action</th>
                    <th>After</th>
                    <th>Last run</th>
                    <th>Affected</th>
                    <th aria-label="Actions" />
                  </tr>
                </thead>
                <tbody>
                  {rules.isPending ? (
                    <SkeletonRows cols={6} />
                  ) : (
                    (rules.data ?? []).map((rule) => {
                      const d = DISPLAY[rule.id];
                      return (
                        <tr key={rule.id}>
                          <td className="mono">{rule.prefix}</td>
                          <td>
                            <Pill tone="mut">{actionLabel(rule.action)}</Pill>
                          </td>
                          <td className="mono">{rule.after_days ?? "—"} d</td>
                          <td className="mono">{d?.last ?? "not yet run"}</td>
                          <td className="mono">{d?.affected ?? "—"}</td>
                          <td>
                            <span className="flex justify-end gap-2">
                              <Btn
                                variant="s"
                                className="h-6 px-2.5 text-10p5"
                                disabled
                                disabledReason="No per-rule resource in the spec (finding)"
                              >
                                Edit
                              </Btn>
                              <Btn
                                variant="s"
                                className="h-6 px-2.5 text-10p5"
                                disabled
                                disabledReason="No per-rule resource in the spec (finding)"
                              >
                                Remove
                              </Btn>
                            </span>
                          </td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>
          </Card>
        )}

        <p className="text-10p5 leading-relaxed text-ink3">
          Rules never race versioning: with versioning on, "delete" writes a delete marker; hard
          removal follows the 30 d soft-delete window.
        </p>
      </div>

      {drawerOpen ? <LifecycleDrawer svc={svc} onClose={() => setDrawerOpen(false)} /> : null}
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/lifecycle")({
  component: LifecyclePage,
});
