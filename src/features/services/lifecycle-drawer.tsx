import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Flabel, Inp } from "@/design-system/inp";
import {
  createLifecycleRuleMutation,
  dryRunLifecycleRuleMutation,
  errorMessage,
  listLifecycleRulesQueryKey,
  type Service,
} from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * U3 · Add lifecycle rule — dry-run first is the drawer's default path
 * (openapi.yaml's own words). The dry-run rides the real
 * lifecycle-rules:dryRun endpoint on open and on every input change,
 * debounced; Add rule posts the real create and invalidates D15's list.
 */

const ACTIONS = [
  { key: "expire" as const, label: "Expire (delete)" },
  { key: "tier_cold" as const, label: "Tier to cold" },
];

export function LifecycleDrawer({ svc, onClose }: { svc: Service; onClose: () => void }) {
  const [prefix, setPrefix] = useState("uploads/tmp/");
  const [after, setAfter] = useState("7 days");
  const [action, setAction] = useState<"expire" | "tier_cold">("expire");
  const queryClient = useQueryClient();
  const dryRun = useMutation(dryRunLifecycleRuleMutation());
  const create = useMutation(createLifecycleRuleMutation());

  const afterDays = Number.parseInt(after, 10) || 7;

  // Dry-run on open and on input change, debounced — mutate is stable in
  // react-query, so the effect re-fires only when the rule's shape changes.
  const dryRunMutate = dryRun.mutate;
  useEffect(() => {
    const t = setTimeout(() => {
      dryRunMutate({ path: { service: svc.id }, body: { prefix, action, after_days: afterDays } });
    }, 350);
    return () => clearTimeout(t);
  }, [dryRunMutate, svc.id, prefix, action, afterDays]);

  // Value mapping: the generated response type says {objects, bytes,
  // monthly_savings_cents} while the canon MSW handler replies {matches,
  // bytes_reclaimed, note} — read both spellings and format like the U3
  // frame's "matches  18,412 objects · 31 GB — none read in 30 d" line.
  const raw = (dryRun.data ?? {}) as Record<string, unknown>;
  const objects = Number(raw.matches ?? raw.objects ?? 0);
  const bytes = Number(raw.bytes_reclaimed ?? raw.bytes ?? 0);
  const gb = (bytes / 2 ** 30).toFixed(1);
  const savingsCents = raw.monthly_savings_cents;
  const note = typeof raw.note === "string" ? raw.note : undefined;

  const submit = () =>
    create.mutate(
      { path: { service: svc.id }, body: { prefix, action, after_days: afterDays } },
      {
        onSuccess: () => {
          toast.success(
            `Lifecycle rule added — ${prefix} · ${action === "expire" ? "delete" : "cold tier"} after ${afterDays} d`,
          );
          queryClient.invalidateQueries({
            queryKey: listLifecycleRulesQueryKey({ path: { service: svc.id } }),
          });
          onClose();
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: scrim dismiss mirrors Esc
    // biome-ignore lint/a11y/useKeyWithClickEvents: Cancel is the keyboard path
    <div className="fixed inset-0 z-40 bg-[rgba(6,9,12,0.44)]" onClick={onClose}>
      {/* biome-ignore lint/a11y/useKeyWithClickEvents: click only stops scrim dismiss */}
      <div
        className="absolute top-0 right-0 flex h-full w-[424px] flex-col border-hair border-l bg-surface shadow-e2"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label="Add lifecycle rule"
      >
        <div className="border-hair border-b px-5 py-4">
          <div className="text-[14px] font-semibold">Add lifecycle rule</div>
          <div className="mono mt-0.5 text-[10.5px] text-ink3">
            {svc.name} · Object Storage · rule 3
          </div>
        </div>

        <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">
          <div>
            <Flabel htmlFor="rule-prefix">Prefix</Flabel>
            <Inp
              id="rule-prefix"
              className="mono"
              value={prefix}
              onChange={(e) => setPrefix(e.target.value)}
            />
          </div>

          <div>
            <Flabel htmlFor="rule-after">After</Flabel>
            <Inp id="rule-after" value={after} onChange={(e) => setAfter(e.target.value)} />
          </div>

          <div className="flex flex-col gap-1.5">
            <Flabel>Action</Flabel>
            <div className="chiprow">
              {ACTIONS.map((a) => (
                <button
                  key={a.key}
                  type="button"
                  className={cn("chip", action === a.key && "on")}
                  onClick={() => setAction(a.key)}
                >
                  {a.label}
                </button>
              ))}
            </div>
          </div>

          <div className="flex flex-col gap-1.5">
            <div className="flex items-baseline">
              <Flabel>Dry run — what this would do today</Flabel>
              <span className="ml-auto text-[10.5px] text-ink3">
                {dryRun.isPending ? "running…" : "ran just now"}
              </span>
            </div>
            <div className="logwell">
              {dryRun.data ? (
                <>
                  <div>
                    <span className="t">matches&nbsp;&nbsp;</span>
                    {objects.toLocaleString("en-US")} objects · {gb} GB — none read in 30 d
                  </div>
                  {typeof savingsCents === "number" ? (
                    <div>
                      <span className="t">savings&nbsp;&nbsp;</span>~$
                      {(savingsCents / 100).toFixed(2)}/mo at current prices
                    </div>
                  ) : null}
                  {note ? (
                    <div>
                      <span className="t">note&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;</span>
                      {note}
                    </div>
                  ) : null}
                </>
              ) : (
                <div className="t">running the dry run…</div>
              )}
            </div>
          </div>

          <p className="text-[11px] leading-relaxed text-ink3">
            Same shape the assistant proposed in <span className="mono">prp_st77a4</span> (AI7) —
            this is the manual path to the identical rule.
          </p>

          <Card className="p-3.5 text-[11px] leading-relaxed text-ink2">
            Runs daily at 03:00 like the existing rules, and every run lands in the event stream —{" "}
            <span className="mono">lifecycle tmp/ removed 2,041 objects</span> style, auditable.
          </Card>
        </div>

        <div className="flex items-center gap-2 border-hair border-t px-5 py-3.5">
          <span className="text-[10.5px] text-ink3">
            Rule 3 of 3 · soft delete still applies 30 d
          </span>
          <span className="flex-1" />
          <Btn variant="s" onClick={onClose}>
            Cancel
          </Btn>
          <Btn variant="p" onClick={submit} disabled={create.isPending}>
            Add rule
          </Btn>
        </div>
      </div>
    </div>
  );
}
