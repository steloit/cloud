import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Glyph } from "@/design-system/glyph";
import { Pill } from "@/design-system/pill";
import { useServices } from "@/features/services/hooks";
import { LifecycleDrawer } from "@/features/services/lifecycle-drawer";
import { type LifecycleRule, listLifecycleRulesOptions } from "@/lib/api";

/**
 * D15 · Lifecycle rules — the table rides the real lifecycle-rules endpoint;
 * last-run and affected counts are frame-fixed display (the schema carries
 * no run history — finding). The D15 frame shows an `uploads/raw/ · 30 d`
 * row, but the canon world data has `originals/ · tier_cold · 90 d` —
 * fixtures-style world wins over the frame.
 */

const DISPLAY: Record<string, { last: string; affected: string }> = {
  "tmp/": { last: "today · 03:00", affected: "2,041 objects" },
  "originals/": { last: "today · 03:00", affected: "11,384 objects" },
};

function actionLabel(action: LifecycleRule["action"]): string {
  return action === "expire" ? "delete" : "→ cold tier";
}

function LifecyclePage() {
  const { service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(env);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const svc = services.data?.find((s) => s.name === service || s.id === service);
  const rules = useQuery({
    ...listLifecycleRulesOptions({ path: { service: svc?.id ?? "" } }),
    select: (r) => r.data ?? [],
    enabled: Boolean(svc),
  });

  if (!svc) return <main className="main" />;
  if (svc.product !== "storage") {
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
        <Pghead
          before={<Glyph id="s-bucket" />}
          title="Lifecycle rules"
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
                {(rules.data ?? []).map((rule) => {
                  const d = DISPLAY[rule.prefix];
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
                })}
              </tbody>
            </table>
          </div>
        </Card>

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
