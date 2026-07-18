import { useMutation } from "@tanstack/react-query";
import { useNavigate, useParams } from "@tanstack/react-router";
import { useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { Icon } from "@/design-system/icon";
import { Kbd } from "@/design-system/kbd";
import { Scrim, useOverlay } from "@/design-system/overlay";
import { Pill } from "@/design-system/pill";
import { useProjects } from "@/features/projects/hooks";
import { useServices } from "@/features/services/hooks";
import { errorMessage, rollbackDeploymentMutation } from "@/lib/api";
import { usePaletteOpen, useSetPaletteOpen, useUIStore } from "@/lib/store";
import { cn } from "@/lib/utils";

/**
 * W7 · ⌘K — jump · act · ask; nothing is more than one palette away.
 * Destructive actions always confirm — palette speed never removes designed
 * friction (the rollback rows require a second ↵).
 */

interface PaletteItem {
  key: string;
  section: "Actions" | "Jump to" | "Ask the assistant";
  title: React.ReactNode;
  sub?: React.ReactNode;
  confirm?: boolean;
  run: () => void;
  keywords: string;
}

export function CommandPalette() {
  const open = usePaletteOpen();
  if (!open) return null;
  // Mounted only while open so useOverlay's document-level Esc listener and
  // focus trap exist only while the palette does; state resets by remount.
  return <Palette />;
}

function Palette() {
  const setOpen = useSetPaletteOpen();
  const openAssistant = useUIStore((s) => s.openAssistant);
  const navigate = useNavigate();
  const params = useParams({ strict: false }) as { org?: string; project?: string };
  const org = params.org ?? "acme";
  const projects = useProjects(org);
  const services = useServices("production");
  const rollback = useMutation(rollbackDeploymentMutation());

  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState(0);
  const [armed, setArmed] = useState<string | null>(null);

  const close = () => setOpen(false);
  const panelRef = useRef<HTMLDivElement>(null);
  useOverlay(panelRef, close);

  const items = useMemo<PaletteItem[]>(() => {
    const project = params.project ?? "ecommerce";
    const jump: PaletteItem[] = [
      ...(projects.data ?? []).map((p) => ({
        key: `prj-${p.id}`,
        section: "Jump to" as const,
        title: (
          <>
            Project · <b>{p.name}</b>
          </>
        ),
        run: () =>
          navigate({
            to: "/$org/$project",
            params: { org, project: p.name },
            search: { env: "production" },
          }),
        keywords: `project ${p.name}`,
      })),
      ...(services.data ?? []).map((s) => ({
        key: `svc-${s.id}`,
        section: "Jump to" as const,
        title: (
          <>
            Service · <b>{s.name}</b>
          </>
        ),
        run: () =>
          navigate({
            to: "/$org/$project/svc/$service",
            params: { org, project, service: s.name },
            search: { env: "production" },
          }),
        keywords: `service ${s.name} ${s.product}`,
      })),
      {
        key: "observe",
        section: "Jump to" as const,
        title: (
          <>
            Observe · <b>Health</b>
          </>
        ),
        run: () =>
          navigate({
            to: "/$org/$project/observe/health",
            params: { org, project },
            search: { env: "production" },
          }),
        keywords: "observe health metrics logs traces alerts events",
      },
      {
        key: "deploy",
        section: "Jump to" as const,
        title: (
          <>
            Deploy · <b>Deployments</b>
          </>
        ),
        run: () =>
          navigate({
            to: "/$org/$project/deploy",
            params: { org, project },
            search: { env: "production" },
          }),
        keywords: "deploy deployments rollout previews promote",
      },
      {
        key: "create",
        section: "Jump to" as const,
        title: (
          <>
            Create · <b>New instance</b>
          </>
        ),
        run: () => navigate({ to: "/$org/create", params: { org }, search: { env: "production" } }),
        keywords: "create new instance add service postgres valkey",
      },
      {
        key: "audit",
        section: "Jump to" as const,
        title: (
          <>
            Settings · <b>Audit log</b>
          </>
        ),
        run: () => navigate({ to: "/$org/settings/audit", params: { org } }),
        keywords: "audit log settings who decided",
      },
    ];

    const actions: PaletteItem[] = [
      {
        key: "rollback-api",
        section: "Actions",
        title: (
          <>
            Roll back <span className="mono">api</span> to deploy <span className="mono">#141</span>
          </>
        ),
        sub: "production · reverts image e4a91c2 → b7d203f · config unchanged · ~40s",
        confirm: true,
        run: () =>
          rollback.mutate(
            { path: { dep: "dep_142" } },
            {
              onSuccess: () => toast.success("Rollback started — api → #141 (~40s)"),
              onError: (err) => toast.error(errorMessage(err)),
            },
          ),
        keywords: "roll back rollback api deploy 141",
      },
      {
        key: "rollback-worker",
        section: "Actions",
        title: (
          <>
            Roll back <span className="mono">worker</span> to <span className="mono">#139</span>
          </>
        ),
        confirm: true,
        run: () =>
          rollback.mutate(
            { path: { dep: "dep_140" } },
            {
              onSuccess: () => toast.success("Rollback started — worker → #139"),
              onError: (err) => toast.error(errorMessage(err)),
            },
          ),
        keywords: "roll back rollback worker deploy 139",
      },
    ];

    const ask: PaletteItem[] = /roll/i.test(query)
      ? [
          {
            key: "ask-rollback",
            section: "Ask the assistant",
            title: <b>"Should I roll back api?"</b>,
            sub: (
              <>
                Latency began 4 min after #142 and correlates with connection saturation from an
                unindexed query the deploy introduced. A rollback will recover latency; the index
                proposal on db-main fixes the cause.{" "}
                <span className="text-ink3">
                  Sources: deploy #142 · db-main query insights · alert AL-291
                </span>
              </>
            ),
            run: () => openAssistant(),
            keywords: "should i roll back api ask",
          },
        ]
      : [];

    return [...actions, ...jump, ...ask];
  }, [projects.data, services.data, params.project, org, navigate, query, rollback, openAssistant]);

  const q = query.trim().toLowerCase();
  const visible = q ? items.filter((i) => i.keywords.toLowerCase().includes(q)) : items;
  const sections = ["Actions", "Jump to", "Ask the assistant"] as const;

  const flat = sections.flatMap((s) => visible.filter((i) => i.section === s));

  const activate = (item: PaletteItem) => {
    if (item.confirm && armed !== item.key) {
      setArmed(item.key);
      return;
    }
    setOpen(false);
    item.run();
  };

  return (
    <Scrim onDismiss={close} className="flex items-start justify-center pt-[15vh]">
      {/* biome-ignore lint/a11y/useKeyWithClickEvents: click-stop only, so the scrim doesn't dismiss; Esc/Tab handled by useOverlay */}
      {/* biome-ignore lint/a11y/noStaticElementInteractions: click-stop only */}
      <div
        ref={panelRef}
        className="w-[640px] overflow-hidden rounded-xl border border-hair bg-surface shadow-e2"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label="Command palette"
      >
        <div className="flex items-center gap-2.5 border-hair border-b px-4 py-3">
          <Icon id="s-search" className="h-4 w-4 text-ink3" />
          <input
            // biome-ignore lint/a11y/noAutofocus: W7 — the palette exists to type into; useOverlay honors [autofocus]
            autoFocus
            className="flex-1 bg-transparent text-13 outline-none placeholder:text-ink3"
            placeholder="Jump, act, or ask…"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setSelected(0);
              setArmed(null);
            }}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown") {
                e.preventDefault();
                setSelected((s) => Math.min(s + 1, flat.length - 1));
              }
              if (e.key === "ArrowUp") {
                e.preventDefault();
                setSelected((s) => Math.max(s - 1, 0));
              }
              if (e.key === "Enter") {
                const item = flat[selected];
                if (item) activate(item);
              }
            }}
          />
          <Kbd>esc</Kbd>
        </div>
        <div className="max-h-[420px] overflow-y-auto p-1.5">
          {sections.map((section) => {
            const group = visible.filter((i) => i.section === section);
            if (group.length === 0) return null;
            return (
              <div key={section}>
                <div className="eyebrow px-3 py-2">{section}</div>
                {group.map((item) => {
                  const idx = flat.indexOf(item);
                  return (
                    <button
                      key={item.key}
                      type="button"
                      className={cn(
                        "flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left",
                        idx === selected && "bg-surface2",
                      )}
                      onMouseEnter={() => setSelected(idx)}
                      onClick={() => activate(item)}
                    >
                      <span className="flex-1">
                        <span className="block text-12p5">{item.title}</span>
                        {item.sub ? (
                          <span className="mt-0.5 block text-10p5 leading-snug text-ink3">
                            {item.sub}
                          </span>
                        ) : null}
                      </span>
                      {item.confirm ? (
                        <Pill tone="warn">
                          {armed === item.key ? "press ↵ again" : "confirm on ↵"}
                        </Pill>
                      ) : null}
                      <Kbd>↵</Kbd>
                    </button>
                  );
                })}
              </div>
            );
          })}
          {flat.length === 0 ? (
            <div className="px-3 py-6 text-center text-12 text-ink3">
              Nothing matches — try a service name, or ask the assistant.
            </div>
          ) : null}
        </div>
        <div className="flex items-center gap-3 border-hair border-t px-4 py-2.5 text-10p5 text-ink3">
          <span>
            <Kbd>↑↓</Kbd> navigate
          </span>
          <span>
            <Kbd>↵</Kbd> select
          </span>
          <span className="ml-auto">
            destructive actions always confirm — palette speed never removes designed friction
          </span>
        </div>
      </div>
    </Scrim>
  );
}
