/**
 * T8.2 / U5 — provisioning entities poll INDEPENDENTLY and survive drawer
 * close: the watch registry lives here (module-scope zustand), and the
 * headless <ProvisioningWatcher/> in the shell owns the polling — a drawer
 * only registers ids. Closing the drawer changes nothing (the AC).
 */

import { useQueries, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { toast } from "sonner";
import { create } from "zustand";
import { getServiceOptions } from "@/lib/api";
import { POLL_MAX_MS, POLL_START_MS } from "@/lib/realtime";

interface WatchState {
  /** Watched provisioning service ids (U5 drawers register here). */
  watched: string[];
  watch: (id: string) => void;
  unwatch: (id: string) => void;
}

export const useWatchStore = create<WatchState>((set) => ({
  watched: [],
  watch: (id) => set((s) => (s.watched.includes(id) ? s : { watched: [...s.watched, id] })),
  unwatch: (id) => set((s) => ({ watched: s.watched.filter((w) => w !== id) })),
}));

/** A drawer calls this on create; nothing else is its concern. */
export function watchProvisioning(serviceId: string) {
  useWatchStore.getState().watch(serviceId);
}

/**
 * Headless — mounted ONCE in the shell. Each watched service polls 2s→10s
 * (state.md backoff) until its status leaves `provisioning`; then: toast,
 * invalidate the lists (status pills flip on server truth), unwatch.
 */
export function ProvisioningWatcher() {
  const watched = useWatchStore((s) => s.watched);
  const unwatch = useWatchStore((s) => s.unwatch);
  const qc = useQueryClient();
  const started = useRef<Map<string, number>>(new Map());

  const results = useQueries({
    queries: watched.map((id) => ({
      ...getServiceOptions({ path: { service: id } }),
      refetchInterval: () => {
        // 2s doubling to 10s, per entity, from when watching began.
        const t0 = started.current.get(id) ?? Date.now();
        if (!started.current.has(id)) started.current.set(id, t0);
        const elapsed = Date.now() - t0;
        const delay = Math.min(POLL_START_MS * 2 ** Math.floor(elapsed / 10_000), POLL_MAX_MS);
        return delay;
      },
    })),
  });

  useEffect(() => {
    results.forEach((r, i) => {
      const id = watched[i];
      if (!id || !r.data) return;
      const svc = r.data as { name?: string; status?: string };
      if (svc.status && svc.status !== "provisioning") {
        // server truth arrived — flip the pills everywhere and say so.
        if (svc.status === "ready") {
          toast.success(`${svc.name ?? "Service"} is ready — metering starts now`);
        } else {
          toast.error(`${svc.name ?? "Service"} entered ${svc.status}`);
        }
        // hey-api keys are structured ({_id} in key[0]) — predicate-match every
        // env's list, since the watcher is env-agnostic by design.
        qc.invalidateQueries({
          predicate: (q) => {
            const head = q.queryKey[0] as { _id?: string } | undefined;
            return head?._id === "listServices" || head?._id === "listDeployments";
          },
        });
        started.current.delete(id);
        unwatch(id);
      }
    });
  }, [results, watched, unwatch, qc]);

  return null;
}
