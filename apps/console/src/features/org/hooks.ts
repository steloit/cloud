import { useQuery } from "@tanstack/react-query";
import {
  getBillingOverviewOptions,
  getOrgOptions,
  listAuditEventsOptions,
  listMyOrgsOptions,
} from "@/lib/api";

export function useOrgs() {
  return useQuery({ ...listMyOrgsOptions(), select: (r) => r.data ?? [] });
}

export function useOrg(org: string) {
  return useQuery(getOrgOptions({ path: { org } }));
}

export function useBillingOverview(org: string) {
  return useQuery(getBillingOverviewOptions({ path: { org } }));
}

export function useAuditEvents(org: string) {
  return useQuery({
    ...listAuditEventsOptions({ path: { org } }),
    select: (r) => r.data ?? [],
  });
}
