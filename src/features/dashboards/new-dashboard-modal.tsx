import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Flabel, Inp } from "@/design-system/inp";
import { Modal, ModalHead } from "@/design-system/overlay";
import { createDashboardMutation, type Visibility } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * DB8 · New dashboard — scope (what it ranges over) and visibility (who can
 * see it) are separate axes; "Start from" picks the initial grid. Parent
 * renders this conditionally; the scrim click and Cancel both close.
 */

const SCOPES = ["org-wide", "ecommerce", "other project ▾"] as const;
type Scope = (typeof SCOPES)[number];

const VISIBILITIES: Visibility[] = ["personal", "org", "restricted"];

const STARTS = [
  { key: "blank", name: "Blank", desc: "an empty grid and the add-widget drawer" },
  {
    key: "template",
    name: "Template — Release Review",
    desc: "deploy markers, error delta, rollback shortcut",
  },
  {
    key: "duplicate",
    name: "Duplicate — checkout-health",
    desc: "your incident dashboard as a base",
  },
] as const;
type StartKey = (typeof STARTS)[number]["key"];

export function NewDashboardModal({ org, onClose }: { org: string; onClose: () => void }) {
  const [name, setName] = useState("release-watch");
  const [scope, setScope] = useState<Scope>("ecommerce");
  const [visibility, setVisibility] = useState<Visibility>("personal");
  const [start, setStart] = useState<StartKey>("template");
  const create = useMutation(createDashboardMutation());

  const submit = () => {
    // Canon-mode create is echo-only: MSW has no POST /orgs/:org/dashboards
    // handler, so the mutation fires for the API contract's sake and success
    // is toasted optimistically instead of awaiting a response.
    create.mutate({
      path: { org },
      body: {
        name,
        scope: scope === "org-wide" ? "org" : "project:prj_ecommerce",
        visibility,
        start_from:
          start === "blank"
            ? "blank"
            : start === "template"
              ? "template:dsh_tpl_release_review"
              : "duplicate:dsh_checkout_health",
      },
    });
    toast.success("Dashboard created");
    onClose();
  };

  return (
    <Modal label="New dashboard" onClose={onClose}>
      <ModalHead title="New dashboard" />

      <div>
        <Flabel htmlFor="dash-name">Name</Flabel>
        <Inp id="dash-name" value={name} onChange={(e) => setName(e.target.value)} />
      </div>

      <div className="flex flex-col gap-1.5">
        <Flabel>Scope — what it ranges over</Flabel>
        <div className="chiprow">
          {SCOPES.map((s) => (
            <button
              key={s}
              type="button"
              className={cn("chip", scope === s && "on")}
              onClick={() => setScope(s)}
            >
              {s}
            </button>
          ))}
        </div>
        <div className="text-10p5 text-ink3">
          Project-scoped is born filtered — it inherits the crumb's environment filter and the
          project's permissions.
        </div>
      </div>

      <div className="flex flex-col gap-1.5">
        <Flabel>Visibility — who can see it</Flabel>
        <div className="chiprow">
          {VISIBILITIES.map((v) => (
            <button
              key={v}
              type="button"
              className={cn("chip", visibility === v && "on")}
              onClick={() => setVisibility(v)}
            >
              {v}
            </button>
          ))}
        </div>
      </div>

      <div className="flex flex-col gap-1.5">
        <Flabel>Start from</Flabel>
        {STARTS.map((s) => (
          <button
            key={s.key}
            type="button"
            className={cn(
              "card p-3 text-left",
              start === s.key ? "border-steel" : "hover:border-ink3",
            )}
            onClick={() => setStart(s.key)}
            aria-pressed={start === s.key}
          >
            <div className="text-12p5 font-medium">{s.name}</div>
            <div className="mt-0.5 text-11 text-ink3">{s.desc}</div>
          </button>
        ))}
      </div>

      <div className="flex items-center justify-end gap-2 border-hair border-t pt-3.5">
        <Btn variant="s" onClick={onClose}>
          Cancel
        </Btn>
        <Btn variant="p" onClick={submit}>
          Create dashboard
        </Btn>
      </div>
    </Modal>
  );
}
