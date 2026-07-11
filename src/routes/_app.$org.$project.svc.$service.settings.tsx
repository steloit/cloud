import { createFileRoute } from "@tanstack/react-router";
import { Card } from "@/design-system/card";
import { useServices } from "@/features/services/hooks";
import { PostgresSettingsTab } from "@/features/services/tabs/postgres";
import { QueueSettingsTab } from "@/features/services/tabs/queue";
import { StorageSettingsTab } from "@/features/services/tabs/storage";
import { ValkeySettingsTab } from "@/features/services/tabs/valkey";
import { WebSettingsTab } from "@/features/services/tabs/web";

/** Per-product Settings tab dispatcher — D12 / D14 / D16 / D18 / D23. */
function SettingsTab() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(env);
  const svc = services.data?.find((s) => s.name === service || s.id === service);
  if (!svc) return <main className="main" />;

  const props = { svc, org, project, env };
  const body =
    svc.product === "postgres" ? (
      <PostgresSettingsTab {...props} />
    ) : svc.product === "valkey" ? (
      <ValkeySettingsTab {...props} />
    ) : svc.product === "storage" ? (
      <StorageSettingsTab {...props} />
    ) : svc.product === "queue" ? (
      <QueueSettingsTab {...props} />
    ) : svc.product === "web" || svc.product === "worker" ? (
      <WebSettingsTab {...props} />
    ) : (
      <Card className="p-4 text-[12px] text-ink2">
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
