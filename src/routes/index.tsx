import { createFileRoute, redirect } from "@tanstack/react-router";
import { listMyOrgsOptions } from "@/lib/api";
import { queryClient } from "@/lib/api/query-client";
import { useSessionStore } from "@/lib/session";

/** "/" — pure redirect: unauthenticated → /login, else the first org's home. */
export const Route = createFileRoute("/")({
  beforeLoad: async ({ location }) => {
    const authed = useSessionStore.getState().session !== null;
    if (!authed) throw redirect({ to: "/login", search: { returnTo: location.href } });
    const orgs = await queryClient.fetchQuery(listMyOrgsOptions());
    const first = orgs.data?.[0];
    if (!first) throw redirect({ to: "/onboarding/org" });
    throw redirect({ to: "/$org", params: { org: first.slug } });
  },
  component: () => null,
});
