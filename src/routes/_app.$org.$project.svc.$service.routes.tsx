import { createFileRoute } from "@tanstack/react-router";
import { Card } from "@/design-system/card";
import { resolveService } from "@/features/services/gateway";
import { GatewayRoutesTab } from "@/features/services/gateway-tabs";
import { useServices } from "@/features/services/hooks";
import { resolveEnvKey } from "@/lib/canon-env";

/** X1 sub-tab — gateway only; other products get the honest one-liner. */
function TabRoute() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));
  const svc = resolveService(services.data, service);
  if (!svc) return <main className="main" />;

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        {svc.product === "ai-gateway" ? (
          <GatewayRoutesTab svc={svc} org={org} project={project} env={env} />
        ) : (
          <Card className="p-4 text-[12px] text-ink2">
            This tab belongs to the AI Gateway capability.
          </Card>
        )}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/routes")({
  component: TabRoute,
});
