import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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

export function useSubscription(org: string) {
  return useQuery(getSubscriptionOptions({ path: { org } }));
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
