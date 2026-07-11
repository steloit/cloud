import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Icon, type IconId } from "@/design-system/icon";
import { Pill } from "@/design-system/pill";
import { useOrgs } from "@/features/org/hooks";
import { cn } from "@/lib/utils";

/**
 * AI3 · the ai-assistant policy as a full page (Law 4): one master switch,
 * the exact surfaces it hides, what it never touches, impact, per-project
 * overrides and audit. The segmented control works locally; Save stays
 * disabled until the enforcement actually changes.
 */

type Enforcement = "Enabled" | "Opt-in only" | "Disabled";
const SEGMENTS: Enforcement[] = ["Enabled", "Opt-in only", "Disabled"];

const HIDES: { icon: IconId; label: string; ref: string }[] = [
  {
    icon: "s-ai",
    label: "Assistant workspace — Ask, Insights, Activity, Capabilities",
    ref: "AI2",
  },
  { icon: "s-ai", label: "Assistant button & quick drawer", ref: "AI4" },
  { icon: "s-plus", label: "Describe-to-provision in Create", ref: "AI1" },
  {
    icon: "s-db",
    label: "Product AI panels — Postgres, Valkey, Storage, Queue",
    ref: "W10 · AI6–8",
  },
  { icon: "s-chart", label: "Insight follow-up conversations", ref: "AI9" },
  { icon: "s-eye", label: "First-run Assistant coachmark", ref: "AI5" },
];

const NEVER_AFFECTS: { label: string; icon: IconId }[] = [
  { label: "Create — the product grid, forms & estimates", icon: "s-plus" },
  { label: "Observe — metrics, logs, traces, events", icon: "s-pulse" },
  { label: "Deploy — promotion, rollout, previews", icon: "s-deploy" },
  { label: "Settings — all manual tuning & config", icon: "s-gear" },
  { label: "Policies, billing, members & everything else", icon: "s-shield" },
];

const OVERRIDES = [
  { project: "ecommerce", pill: { tone: "ok" as const, label: "on" } },
  { project: "analytics-pipeline", pill: { tone: "ok" as const, label: "on" } },
  { project: "internal-tools", pill: { tone: "mut" as const, label: "opted out" } },
];

function AiAssistantPolicyPage() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const [saved, setSaved] = useState<Enforcement>("Enabled");
  const [enforcement, setEnforcement] = useState<Enforcement>("Enabled");
  const dirty = enforcement !== saved;

  const save = () => {
    setSaved(enforcement);
    toast.success("Policy updated — audited as you");
  };

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project="ecommerce"
        active="o-policies"
      />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title={
              <span className="flex items-center gap-2">
                <span className="mono text-[18px]">ai-assistant</span>
                <Pill tone="ai">AI</Pill>
              </span>
            }
            sub="The master switch for every AI feature. When off, the AI layer disappears cleanly and the platform keeps working — AI is an enhancement, never a dependency."
          >
            <Pill tone="ok">enabled</Pill>
            <Btn
              variant="p"
              onClick={save}
              disabled={!dirty}
              disabledReason="Nothing changed — pick a different enforcement first"
            >
              Save changes
            </Btn>
          </Pghead>

          <Card className="p-4">
            <Eyebrow className="mb-2.5">Enforcement · organization-wide</Eyebrow>
            <div className="inline-flex overflow-hidden rounded-[9px] border border-hair">
              {SEGMENTS.map((seg, i) => (
                <button
                  key={seg}
                  type="button"
                  aria-pressed={enforcement === seg}
                  className={cn(
                    "cursor-pointer px-[18px] py-2 text-[12px] text-ink2",
                    i > 0 && "border-hair border-l",
                    enforcement === seg && "bg-ok-tint font-semibold text-ok",
                  )}
                  onClick={() => setEnforcement(seg)}
                >
                  {seg}
                </button>
              ))}
            </div>
            <div className="mt-2.5 text-[10.5px] leading-relaxed text-ink2">
              <b className="text-ok">Enabled</b> — AI is available to everyone in the org;
              individual projects may opt out.{" "}
              <span className="text-ink3">
                Opt-in only = off by default, projects enable it. Disabled = off everywhere and
                cannot be turned on.
              </span>
            </div>
          </Card>

          <div className="flex gap-3">
            <Card className="flex flex-1 flex-col p-4">
              <div className="mb-1.5 flex items-center gap-2">
                <Icon id="s-ai" className="h-[13px] w-[13px] text-assist" />
                <b className="text-[11.5px]">Turning this off hides</b>
              </div>
              <div className="mb-2 text-[9.5px] text-ink3">
                Every AI surface disappears at once — no half-states.
              </div>
              {HIDES.map((row) => (
                <div
                  key={row.ref}
                  className="flex items-center gap-2 border-hair border-b py-[7px] last:border-b-0"
                >
                  <span className="glyph h-[22px] w-[22px]">
                    <Icon id={row.icon} className="h-[11px] w-[11px]" />
                  </span>
                  <span className="flex-1 text-[11px] text-ink1">{row.label}</span>
                  <span className="mono text-[8.5px] text-ink3">{row.ref}</span>
                </div>
              ))}
            </Card>

            <Card className="flex flex-1 flex-col p-4">
              <div className="mb-1.5 flex items-center gap-2">
                <Icon id="s-shield" className="h-[13px] w-[13px] text-ok" />
                <b className="text-[11.5px]">It never affects</b>
              </div>
              <div className="mb-2 text-[9.5px] text-ink3">
                The platform is whole without AI — nothing here changes.
              </div>
              {NEVER_AFFECTS.map((row) => (
                <div
                  key={row.label}
                  className="flex items-center gap-2 border-hair border-b py-[7px] last:border-b-0"
                >
                  <Icon id="s-check" className="h-[13px] w-[13px] text-ok" />
                  <span className="flex-1 text-[11px] text-ink1">{row.label}</span>
                  <Icon id={row.icon} className="h-3 w-3 text-ink3" />
                </div>
              ))}
            </Card>

            <div className="flex w-[280px] shrink-0 flex-col gap-3">
              <Card className="border-warn/35 p-3.5">
                <Eyebrow className="mb-2">If you disable now</Eyebrow>
                <div className="flex flex-col gap-1.5 text-[10.5px] text-ink2">
                  <div className="flex justify-between">
                    <span>Members affected</span>
                    <b>12</b>
                  </div>
                  <div className="flex justify-between">
                    <span>Open insights hidden</span>
                    <b>3</b>
                  </div>
                  <div className="flex justify-between">
                    <span>In-flight proposals</span>
                    <b>kept, unapplied</b>
                  </div>
                  <div className="flex justify-between">
                    <span>Data deleted</span>
                    <b className="text-ok">none</b>
                  </div>
                </div>
                <div className="mt-2.5 border-hair border-t pt-2 text-[9.5px] text-ink3">
                  Applied changes stay applied. Re-enabling restores every surface instantly.
                </div>
              </Card>
              <Card className="p-3.5">
                <Eyebrow className="mb-2">Per-project overrides</Eyebrow>
                <div className="flex flex-col gap-1.5 text-[10.5px]">
                  {OVERRIDES.map((o) => (
                    <div key={o.project} className="flex justify-between">
                      <span className="text-ink2">{o.project}</span>
                      <Pill tone={o.pill.tone}>{o.pill.label}</Pill>
                    </div>
                  ))}
                </div>
                <div className="mt-2 text-[9px] text-ink3">
                  Project admins set these under the project's Policies.
                </div>
              </Card>
            </div>
          </div>

          <div className="flex items-center gap-2 border-hair border-t pt-2.5 text-[10px] text-ink3">
            <Icon id="s-doc" className="h-3 w-3" />
            <span>
              Enabled since org creation · last reviewed by <b>asha</b> Mar 3 (
              <span className="mono text-[9px]">evt_31c0</span>). Changes take effect immediately
              and are audited. Owner or Admin required.
            </span>
          </div>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/settings/policies/ai-assistant")({
  component: AiAssistantPolicyPage,
});
