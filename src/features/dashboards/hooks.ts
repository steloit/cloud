import { useQuery } from "@tanstack/react-query";
import { getDashboardOptions, listDashboardsOptions } from "@/lib/api";

/**
 * Dashboards data layer (DB1–DB8) — thin wrappers over the generated query
 * helpers. Pre-built dashboards are generated views, never files; only
 * user-made dashboards cross this API (dsh_checkout_health in canon).
 */

export function useDashboards(org: string) {
  return useQuery({
    ...listDashboardsOptions({ path: { org } }),
    select: (r) => r.data ?? [],
  });
}

export function useDashboard(dash: string) {
  return useQuery(getDashboardOptions({ path: { dash } }));
}
