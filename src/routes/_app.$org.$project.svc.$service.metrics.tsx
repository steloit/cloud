import { createFileRoute } from "@tanstack/react-router";
import { Card } from "@/design-system/card";
import { resolveService } from "@/features/services/gateway";
import { useServices } from "@/features/services/hooks";
import { PostgresMetricsTab } from "@/features/services/tabs/postgres";
import { ValkeyMetricsTab } from "@/features/services/tabs/valkey";
import { resolveEnvKey } from "@/lib/canon-env";

/** Per-product Metrics tab dispatcher — D9 (postgres) and D13 (valkey) have frames. */
function MetricsTab() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));
  const svc = resolveService(services.data, service);
  if (!svc) return <main className="main" />;

  const props = { svc, org, project, env };
  const body =
    svc.product === "postgres" ? (
      <PostgresMetricsTab {...props} />
    ) : svc.product === "valkey" ? (
      <ValkeyMetricsTab {...props} />
    ) : (
      <Card className="p-4 text-[12px] text-ink2">
        Metrics for {svc.product} follow the D9 grammar — the gallery has no dedicated frame; this
        pane lands with per-product telemetry.
      </Card>
    );

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">{body}</div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/metrics")({
  component: MetricsTab,
});
