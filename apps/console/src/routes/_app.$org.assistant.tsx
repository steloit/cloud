import { createFileRoute, Outlet, useChildMatches } from "@tanstack/react-router";
import { type AssistantLens, SnavAssistant } from "@/app/shell/snav-assistant";
import { useOrgs } from "@/features/org/hooks";

/**
 * Assistant workspace layout (AI2/AI10/AI11): the lens snav + the selected
 * page. Org-scoped — 02-ia says /assistant/* top-level, but the frames scope
 * to org/project; org-scoped chosen so the shell chrome works (finding,
 * see snav-assistant.tsx).
 */
function AssistantLayout() {
  const { org } = Route.useParams();
  // Pass-6: the snav header names the org — derived via the standard useOrgs
  // lookup (slug or id), never hardcoded; the slug stands in until it resolves.
  const orgs = useOrgs();
  const orgName = orgs.data?.find((o) => o.slug === org || o.id === org)?.name;
  const childMatches = useChildMatches();

  const leaf = childMatches[childMatches.length - 1]?.routeId ?? "";
  const active: AssistantLens = leaf.includes("insights")
    ? "insights"
    : leaf.includes("activity")
      ? "activity"
      : leaf.includes("capabilities")
        ? "capabilities"
        : "ask";

  return (
    <>
      <SnavAssistant org={org} orgName={orgName} active={active} />
      <Outlet />
    </>
  );
}

export const Route = createFileRoute("/_app/$org/assistant")({
  component: AssistantLayout,
});
