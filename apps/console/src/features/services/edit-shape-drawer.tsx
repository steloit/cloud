import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Drawer } from "@/design-system/overlay";
import type { Service } from "@/lib/api";
import { errorMessage, updateServiceMutation } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * Shared "Edit shape…" drawer (overlay census) — the inline resize chips on
 * D12 (postgres size) and D14 (valkey memory) move into one
 * product-parameterized 424 sheet: current shape, target options with the
 * canon prices, the billing delta, and the consequence stated before Apply.
 */

export interface ShapeOption {
  id: string;
  /** "Standard" · "1 GB" — the option's display name. */
  title: string;
  /** "2 vCPU / 4 GB" — omitted where the title already says it all. */
  spec?: string;
  /** Canon $/mo (create-flow type blocks). */
  price: number;
  /** Merged over the service's existing shape on Apply. */
  patch: Record<string, unknown>;
}

export function EditShapeDrawer({
  svc,
  env,
  options,
  currentId,
  onClose,
}: {
  svc: Service;
  env: string;
  options: ShapeOption[];
  currentId: string;
  onClose: () => void;
}) {
  const [picked, setPicked] = useState(currentId);
  const update = useMutation(updateServiceMutation());
  const current = options.find((o) => o.id === currentId);
  const target = options.find((o) => o.id === picked);
  const changed = picked !== currentId;
  const delta = target && current ? target.price - current.price : 0;

  const apply = () => {
    if (!target) return;
    update.mutate(
      {
        path: { service: svc.id },
        body: { shape: { ...((svc.shape ?? {}) as Record<string, unknown>), ...target.patch } },
      },
      {
        // PATCH /services/:service is the echo handler (DB8 add-widget
        // precedent): the merge comes back once, fixtures win on refetch.
        onSuccess: () => {
          toast.success(`${svc.name} → ${target.title} — rolling restart, replicas first (~40 s)`);
          onClose();
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    <Drawer
      title="Edit shape"
      sub={
        <>
          {svc.name} · {env}
        </>
      }
      onClose={onClose}
      footer={
        <>
          <span className="flex-1" />
          <Btn variant="s" onClick={onClose}>
            Cancel
          </Btn>
          <Btn
            variant="p"
            onClick={apply}
            disabled={!changed || update.isPending}
            disabledReason={!changed ? "Pick a different shape" : "Applying the new shape…"}
          >
            Apply
          </Btn>
        </>
      }
    >
      <Card className="flex flex-col gap-2 p-3.5">
        <Eyebrow>Current shape</Eyebrow>
        <div className="flex items-center gap-2 text-12">
          <span>{current?.title}</span>
          {current?.spec ? <span className="text-ink3">· {current.spec}</span> : null}
          <span className="flex-1" />
          <span className="mono">${current?.price}/mo</span>
        </div>
      </Card>

      <div className="flex flex-col gap-1.5">
        <div className="eyebrow">Target shape</div>
        {options.map((o) => (
          <label
            key={o.id}
            className={cn(
              "flex cursor-pointer items-center gap-2.5 rounded-lg border border-hair px-3 py-2",
              picked === o.id && "border-steel",
            )}
          >
            <input
              type="radio"
              name="shape-target"
              checked={picked === o.id}
              onChange={() => setPicked(o.id)}
            />
            <span className="text-12">{o.title}</span>
            {o.spec ? <span className="text-10p5 text-ink3">{o.spec}</span> : null}
            <span className="flex-1" />
            <span className="mono text-11">
              ${o.price}/mo{o.id === currentId ? " · current" : ""}
            </span>
          </label>
        ))}
      </div>

      {changed ? (
        <p className="mono text-11">
          {delta >= 0 ? "+" : "−"}${Math.abs(delta)}/mo from the 1st
        </p>
      ) : null}

      <Card className="p-3.5">
        <p className="text-11p5 leading-relaxed text-ink2">
          Applies with a rolling restart — replicas first, ~40 s. No data moves and bindings stay
          valid throughout.
        </p>
      </Card>
    </Drawer>
  );
}
