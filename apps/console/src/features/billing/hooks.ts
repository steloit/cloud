import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { Subscription } from "@/lib/api";
import {
  cancelSubscriptionMutation,
  changePlanMutation,
  getBillingOverviewQueryKey,
  getQuotasOptions,
  getSubscriptionOptions,
  getSubscriptionQueryKey,
  getUsageOptions,
  listInvoicesOptions,
  listPaymentMethodsOptions,
} from "@/lib/api";

/** Billing-plane data hooks (B1–B12) — thin wrappers over the generated client. */

export function useUsage(org: string) {
  return useQuery(getUsageOptions({ path: { org } }));
}

export function useQuotas(org: string) {
  return useQuery({ ...getQuotasOptions({ path: { org } }), select: (r) => r.data ?? [] });
}

export function useInvoices(org: string) {
  return useQuery({ ...listInvoicesOptions({ path: { org } }), select: (r) => r.data ?? [] });
}

export function useSubscription(org: string, state?: string) {
  const options = getSubscriptionOptions({ path: { org } });
  // `state` is the canon-mode demo affordance (?state=dunning_day8 → B10) —
  // a plain query alongside the generated one, so the strict key type stays.
  const demo = useQuery<Subscription>({
    queryKey: ["subscription-demo", org, state ?? ""],
    enabled: Boolean(state),
    queryFn: async () => {
      const res = await fetch(`/v1/orgs/${org}/subscription?state=${state}`);
      return (await res.json()) as Subscription;
    },
  });
  const live = useQuery({ ...options, enabled: !state });
  return state ? demo : live;
}

export function useChangePlan(org: string) {
  const queryClient = useQueryClient();
  return useMutation({
    ...changePlanMutation(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: getSubscriptionQueryKey({ path: { org } }) });
      queryClient.invalidateQueries({ queryKey: getBillingOverviewQueryKey({ path: { org } }) });
    },
  });
}

export function useCancelPlan(org: string) {
  const queryClient = useQueryClient();
  return useMutation({
    ...cancelSubscriptionMutation(),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: getSubscriptionQueryKey({ path: { org } }) }),
  });
}

export function usePaymentMethods(org: string) {
  return useQuery({
    ...listPaymentMethodsOptions({ path: { org } }),
    select: (r) => r.data ?? [],
  });
}

/** "business" → "Business" — the snav footer's plan label. */
export function planLabel(plan: string): string {
  return plan.charAt(0).toUpperCase() + plan.slice(1);
}
