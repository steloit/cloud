import { createFileRoute, Outlet, useChildMatches } from "@tanstack/react-router";
import { type ObserveLens, SnavObserve } from "@/app/shell/snav-observe";

/**
 * Observe layout (O1–O6): the lens snav + the selected page. No index route —
 * the rail links straight to /observe/health; env arrives via ?env= only
 * (ADR-013).
 */
function ObserveLayout() {
  const { org, project } = Route.useParams();
  const { env } = Route.useSearch();
  const childMatches = useChildMatches();

  const leaf = childMatches[childMatches.length - 1]?.routeId ?? "";
  const active: ObserveLens = leaf.includes("metrics")
    ? "metrics"
    : leaf.includes("logs")
      ? "logs"
      : leaf.includes("traces")
        ? "traces"
        : leaf.includes("events")
          ? "events"
          : leaf.includes("alerts")
            ? "alerts"
            : "health";

  return (
    <>
      <SnavObserve org={org} project={project} env={env} active={active} />
      <Outlet />
    </>
  );
}

export const Route = createFileRoute("/_app/$org/$project/observe")({
  validateSearch: (search: Record<string, unknown>): { env: string } => ({
    env: typeof search.env === "string" ? search.env : "production",
  }),
  component: ObserveLayout,
});
