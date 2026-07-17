import { createFileRoute } from "@tanstack/react-router";
import { resolveService } from "@/features/services/gateway";
import { useServices } from "@/features/services/hooks";
import { BindingsTab } from "@/features/services/tabs/bindings-tab";
import { resolveEnvKey } from "@/lib/canon-env";

/** D11 — bindings are one grammar for every product. */
function BindingsRoute() {
  const { org, project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));
  const svc = resolveService(services.data, service);
  if (!svc) return <main className="main" />;

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <BindingsTab svc={svc} org={org} project={project} env={env} />
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/bindings")({
  component: BindingsRoute,
});
