import { useQuery } from "@tanstack/react-query";
import {
  getServiceOptions,
  listBindingsOptions,
  listDeploymentsOptions,
  listEventsOptions,
  listServicesOptions,
} from "@/lib/api";

/**
 * env is part of every project-scoped key (env-as-filter, ADR-013): the
 * generated query keys embed the {env} path param, so switching the env pill
 * re-fetches without any route change beyond ?env=.
 */
export function useServices(env: string) {
  return useQuery({ ...listServicesOptions({ path: { env } }), select: (r) => r.data ?? [] });
}

export function useService(service: string) {
  return useQuery(getServiceOptions({ path: { service } }));
}

export function useBindings(service: string) {
  return useQuery({
    ...listBindingsOptions({ path: { service } }),
    select: (r) => r.data ?? [],
  });
}

export function useEvents(env: string) {
  return useQuery({ ...listEventsOptions({ path: { env } }), select: (r) => r.data ?? [] });
}

export function useDeployments(env: string) {
  return useQuery({
    ...listDeploymentsOptions({ path: { env } }),
    select: (r) => r.data ?? [],
  });
}
