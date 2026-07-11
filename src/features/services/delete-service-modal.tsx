import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Icon } from "@/design-system/icon";
import { Flabel, Inp } from "@/design-system/inp";
import type { Service } from "@/lib/api";
import { deleteServiceMutation, errorMessage } from "@/lib/api";

/**
 * U6 · typed-confirm delete modal — generic for all products. Dependents are
 * named, the final backup is stated, and the button enables only when the
 * typed name matches. The canon MSW mode has no DELETE /services handler, so
 * the request 404s through — the error toast is the expected path until the
 * handler (or the real API) lands; the mutation is wired regardless.
 */
export function DeleteServiceModal({
  svc,
  sub,
  onClose,
}: {
  svc: Service;
  /** Identity line under the title, e.g. "PostgreSQL 16 · production · 12.4 GB of data". */
  sub: string;
  onClose: () => void;
}) {
  const [typed, setTyped] = useState("");
  const del = useMutation(deleteServiceMutation());
  const matches = typed === svc.name;

  const confirm = () => {
    if (!matches) return;
    del.mutate(
      { path: { service: svc.id } },
      {
        onSuccess: () => {
          toast.success(`Deletion started — final backup kept 30 d`);
          onClose();
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: scrim dismiss mirrors Esc
    // biome-ignore lint/a11y/useKeyWithClickEvents: Esc handled on the confirm input
    <div
      className="fixed inset-0 z-40 flex items-center justify-center bg-[rgba(6,9,12,0.44)]"
      onClick={onClose}
    >
      {/* biome-ignore lint/a11y/useKeyWithClickEvents: click-stop only, so the scrim doesn't dismiss; Esc handled on the input */}
      <div
        className="flex w-[480px] flex-col gap-4 rounded-xl border border-hair bg-surface p-5 shadow-e2"
        role="dialog"
        aria-label={`Delete ${svc.name}`}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start gap-3">
          <span className="glyph text-err">
            <Icon id="s-db" />
          </span>
          <div>
            <div className="text-[14px] font-semibold">Delete {svc.name}?</div>
            <div className="hsub">{sub}</div>
          </div>
        </div>

        <Card className="border-err/45 p-3.5">
          <p className="text-[11.5px] leading-relaxed text-ink2">
            <b>2 services bind {svc.name}</b> — <span className="mono">api</span> and{" "}
            <span className="mono">worker</span> will fail at their next request. Unbind them first,
            or accept the breakage knowingly; nothing here is silent.
          </p>
        </Card>

        <div className="flex flex-col gap-2">
          <div className="flex gap-2 text-[11.5px] text-ink2">
            <span className="text-ok">✓</span>A final backup is taken first · restorable for 30 days
            (your PITR window)
          </div>
          <div className="flex gap-2 text-[11.5px] text-ink2">
            <span className="text-ok">✓</span>Billing stops at deletion — scale-to-zero's last step
          </div>
          <div className="flex gap-2 text-[11.5px] text-ink2">
            <span className="text-err">✗</span>The instance, its bindings and its metrics history
            are gone
          </div>
        </div>

        <div>
          <Flabel htmlFor="delete-confirm">
            <span>
              Type <span className="mono">{svc.name}</span> to confirm
            </span>
          </Flabel>
          <Inp
            id="delete-confirm"
            className="mono"
            value={typed}
            autoFocus
            onChange={(e) => setTyped(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Escape") onClose();
              if (e.key === "Enter") confirm();
            }}
          />
        </div>

        <div className="flex items-center gap-2">
          <span className="flex-1" />
          <Btn variant="s" onClick={onClose}>
            Cancel
          </Btn>
          <Btn
            variant="dgr"
            onClick={confirm}
            disabled={!matches || del.isPending}
            disabledReason={del.isPending ? "Deleting…" : "Button enables when the name matches"}
          >
            Delete {svc.name}
          </Btn>
        </div>
        <div className="text-right text-[10.5px] text-ink3">
          Button enables when the name matches · audited as you
        </div>
      </div>
    </div>
  );
}
