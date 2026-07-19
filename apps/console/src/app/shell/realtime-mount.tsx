import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { toast } from "sonner";
import { useNotificationsStore } from "@/features/notifications/data";
import { resolveEnvKey } from "@/lib/canon-env";
import { VITE_API_URL } from "@/lib/env";
import { createRealtimeChannel, type SpineEvent } from "@/lib/realtime";
import { ProvisioningWatcher } from "@/lib/watch";

/**
 * T8.2 — one realtime channel for the active env (state.md: the stream drives
 * toasts, the bell badge, and status-pill flips). Headless; mounted in the
 * org shell beside the ProvisioningWatcher. In canon mode the channel degrades
 * to polling by construction (MSW cannot stream) — no special case.
 */
export function RealtimeMount({ project, env }: { project?: string; env?: string }) {
  const qc = useQueryClient();
  const bumpLive = useNotificationsStore((s) => s.bumpLive);

  useEffect(() => {
    if (!project || !env) return;
    let invalidateQueued = false;
    const envKey = resolveEnvKey(project, env);
    const channel = createRealtimeChannel({
      env: envKey,
      // live mode: the SSE URL must hit the API origin, not the SPA origin
      // (review H2 — the generated client already does; the stream must too).
      baseUrl: VITE_API_URL,
      onEvent: (e: SpineEvent) => {
        // toasts: deploys + lifecycle edges say so (state.md "realtime").
        if (e.kind === "deploy" && e.action) {
          toast(e.action.replace(".", " "), { description: e.subject });
        }
        if (e.kind === "lifecycle" && e.action === "service.ready") {
          toast.success("Service ready — metering starts now");
        }
        // status pills flip on server truth. COALESCED per microtask batch
        // (review: one invalidation pass per delivered page, not per event).
        if (!invalidateQueued) {
          invalidateQueued = true;
          queueMicrotask(() => {
            invalidateQueued = false;
            qc.invalidateQueries({
              predicate: (q) => {
                const head = q.queryKey[0] as { _id?: string } | undefined;
                return (
                  head?._id === "listServices" ||
                  head?._id === "listDeployments" ||
                  head?._id === "listEvents"
                );
              },
            });
          });
        }
        // the bell badge bumps for every spine event (N1 groups it later —
        // recorded finding: notification-worthy filtering rides the N1 slice).
        bumpLive();
      },
    });
    return () => channel.close();
  }, [project, env, qc, bumpLive]);

  return <ProvisioningWatcher />;
}
