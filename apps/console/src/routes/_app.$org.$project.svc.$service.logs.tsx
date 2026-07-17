import { createFileRoute } from "@tanstack/react-router";
import { Card } from "@/design-system/card";
import { resolveService } from "@/features/services/gateway";
import { useServices } from "@/features/services/hooks";
import { PostgresLogsTab } from "@/features/services/tabs/postgres";
import { resolveEnvKey } from "@/lib/canon-env";

/** Per-product Logs tab dispatcher — D10 (postgres) is the framed exemplar. */
function LogsTab() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));
  const svc = resolveService(services.data, service);
  if (!svc) return <main className="main" />;

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        {svc.product === "postgres" ? (
          <PostgresLogsTab svc={svc} org={org} project={project} env={env} />
        ) : (
          <Card className="p-4 text-12 text-ink2">
            Logs for {svc.product} follow the D10 grammar — the gallery has no dedicated frame; the
            env-wide stream lives in Observe · Logs.
          </Card>
        )}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/logs")({
  component: LogsTab,
});
