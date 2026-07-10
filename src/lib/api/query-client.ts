import { QueryClient } from "@tanstack/react-query";
import { ProblemError } from "./config";

/**
 * Cache policy from 10-state-management/state.md: staleTime 30s for lists
 * (5s for metrics, set per-query); idempotent GETs retry ×3 with exponential
 * backoff; mutations never auto-retry — errors surface with a manual retry.
 */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: (failureCount, error) => {
        if (error instanceof ProblemError && error.problem.status < 500) return false;
        return failureCount < 3;
      },
    },
    mutations: { retry: false },
  },
});
