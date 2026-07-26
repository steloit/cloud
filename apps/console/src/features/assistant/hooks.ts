import { useMutation, useQuery } from "@tanstack/react-query";
import {
  applyProposalMutation,
  createThreadMutation,
  getProposalOptions,
  listInsightsOptions,
  listThreadsOptions,
  postMessageMutation,
  updateInsightMutation,
} from "@/lib/api";

/**
 * Assistant data layer (AI2–AI11) — thin wrappers over the generated helpers.
 * Every write here is a *suggestion-plane* write: threads, messages, insight
 * snooze/dismiss, proposal apply — all audited as "you, via assistant".
 */

export function useThreads() {
  return useQuery({ ...listThreadsOptions(), select: (r) => r.data ?? [] });
}

export function useCreateThread() {
  return useMutation(createThreadMutation());
}

/** Messages post to one thread; the path param is pre-bound so call sites send content only. */
export function usePostMessage(thread: string) {
  return useMutation(postMessageMutation({ path: { thread } }));
}

export function useInsights(filters?: {
  category?: "reliability" | "performance" | "cost" | "security";
  status?: "open" | "applied" | "dismissed" | "snoozed";
}) {
  return useQuery({
    ...listInsightsOptions(filters ? { query: filters } : undefined),
    select: (r) => r.data ?? [],
  });
}

export function useUpdateInsight() {
  return useMutation(updateInsightMutation());
}

export function useProposal(prp: string) {
  return useQuery(getProposalOptions({ path: { prp } }));
}

export function useApplyProposal() {
  return useMutation(applyProposalMutation());
}
