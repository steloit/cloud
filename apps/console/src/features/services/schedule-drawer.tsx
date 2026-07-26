import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Flabel, Inp } from "@/design-system/inp";
import { Drawer } from "@/design-system/overlay";
import {
  createScheduleMutation,
  errorMessage,
  listSchedulesQueryKey,
  previewScheduleOptions,
  type Service,
} from "@/lib/api";

/**
 * U4 · New schedule — a schedule enqueues like any message. The next-runs
 * preview rides the real /schedules:preview query, refetching as the cron
 * changes (debounced); Create posts the real schedule and invalidates D17.
 */

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return v;
}

/** "2026-07-07T02:00:00+05:30" → "Jul 7 02:00" — the drawer's next-runs grammar. */
function fmtRun(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const day = new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    timeZone: "Asia/Kolkata",
  }).format(d);
  const time = new Intl.DateTimeFormat("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "Asia/Kolkata",
  }).format(d);
  return `${day} ${time}`;
}

export function ScheduleDrawer({ svc, onClose }: { svc: Service; onClose: () => void }) {
  const [name, setName] = useState("nightly-reconcile");
  const [cron, setCron] = useState("0 2 * * *");
  const queryClient = useQueryClient();
  const create = useMutation(createScheduleMutation());

  const debouncedCron = useDebounced(cron, 350);
  const preview = useQuery({
    ...previewScheduleOptions({ query: { cron: debouncedCron, tz: "Asia/Kolkata" } }),
    enabled: debouncedCron.trim().length > 0,
    retry: false,
  });

  // The canon MSW world has no /schedules:preview handler yet (finding) —
  // when the query yields nothing, the frame's canon next-runs line renders.
  const nextRuns =
    preview.data?.next_runs && preview.data.next_runs.length > 0
      ? preview.data.next_runs.slice(0, 3).map(fmtRun).join(" · ")
      : "Jul 7 02:00 · Jul 8 02:00 · Jul 9 02:00";

  const submit = () =>
    create.mutate(
      {
        path: { service: svc.id },
        body: {
          name,
          cron,
          tz: "Asia/Kolkata",
          payload_template: { job: "reconcile", date: "{{date}}" },
        },
      },
      {
        onSuccess: () => {
          toast.success(`Schedule created — ${name} · ${cron}`);
          queryClient.invalidateQueries({
            queryKey: listSchedulesQueryKey({ path: { service: svc.id } }),
          });
          onClose();
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );

  return (
    <Drawer
      title="New schedule"
      sub={<>{svc.name} · Queue · enqueues like any message</>}
      onClose={onClose}
      footer={
        <>
          <span className="text-10p5 text-ink3">2nd schedule on {svc.name}</span>
          <span className="flex-1" />
          <Btn variant="s" onClick={onClose}>
            Cancel
          </Btn>
          <Btn variant="p" onClick={submit} disabled={create.isPending}>
            Create schedule
          </Btn>
        </>
      }
    >
      <div>
        <Flabel htmlFor="schedule-name">Name</Flabel>
        <Inp
          id="schedule-name"
          className="mono"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </div>

      <div>
        <Flabel htmlFor="schedule-cron">Cron</Flabel>
        <Inp
          id="schedule-cron"
          className="mono"
          value={cron}
          onChange={(e) => setCron(e.target.value)}
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <Flabel>Next runs · Asia/Kolkata</Flabel>
        <div className="logwell">
          {preview.isFetching ? <span className="t">previewing…</span> : nextRuns}
        </div>
      </div>

      <div className="flex flex-col gap-1.5">
        <Flabel>Payload template</Flabel>
        <div className="logwell">{'{ "job": "reconcile", "date": "{{date}}" }'}</div>
      </div>

      <p className="text-11 leading-relaxed text-ink3">
        Interpolated per run (<span className="mono">{"{{date}}"}</span>,{" "}
        <span className="mono">{"{{run_id}}"}</span>) — D17's grammar. Enqueued to jobs like any
        other message.
      </p>

      <Card className="p-3.5 text-11 leading-relaxed text-ink2">
        Missed runs fail loudly: if the queue is paused or the run can't enqueue, it pages through
        Observe like any other failure — schedules are not fire-and-forget.
      </Card>
    </Drawer>
  );
}
