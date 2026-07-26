import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Card } from "@/design-system/card";
import { PermissionDenied } from "@/features/errors/failure-states";
import { useServices } from "@/features/services/hooks";
import { resolveService } from "@/features/services/resolve";
import { PostgresSettingsTab } from "@/features/services/tabs/postgres";
import { QueueSettingsTab } from "@/features/services/tabs/queue";
import { StorageSettingsTab } from "@/features/services/tabs/storage";
import { ValkeySettingsTab } from "@/features/services/tabs/valkey";
import { WebSettingsTab } from "@/features/services/tabs/web";
import type { CanonProduct } from "@/lib/api/legacy";
import { resolveEnvKey } from "@/lib/canon-env";
import { useSession } from "@/lib/session";

/** Per-product Settings tab dispatcher — D12 / D14 / D16 / D18 / D23. */
function SettingsTab() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));
  const session = useSession();
  const navigate = useNavigate();
  const svc = resolveService(services.data, service);
  if (!svc) return <main className="main" />;

  // E3: Developers can operate services but not change their configuration
  // (project policy prod-guard) — the denial names its grammar, never a 500.
  if (session?.role === "developer") {
    return (
      <main className="main">
        <div className="pgpad">
          <PermissionDenied
            need="Admin"
            page="view service settings"
            role={`${session.name.toLowerCase().split(" ")[0]} (Developer)`}
            permission="project.service.settings"
            onBack={() =>
              navigate({
                to: "/$org/$project/svc/$service",
                params: { org, project, service: svc.name },
                search: { env },
              })
            }
          />
        </div>
      </main>
    );
  }

  const props = { svc, org, project, env };
  const body =
    svc.product === "postgres" ? (
      <PostgresSettingsTab {...props} />
    ) : svc.product === "valkey" ? (
      <ValkeySettingsTab {...props} />
    ) : (svc.product as CanonProduct) === "storage" ? (
      <StorageSettingsTab {...props} />
    ) : (svc.product as CanonProduct) === "queue" ? (
      <QueueSettingsTab {...props} />
    ) : svc.product === "web" || svc.product === "worker" ? (
      <WebSettingsTab {...props} />
    ) : (
      <Card className="p-4 text-12 text-ink2">
        Settings for {svc.product} land in a later phase.
      </Card>
    );

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">{body}</div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/settings")({
  component: SettingsTab,
});
