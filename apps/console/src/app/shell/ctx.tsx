import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { Icon } from "@/design-system/icon";
import { Kbd } from "@/design-system/kbd";
import { Dot, healthDotTone } from "@/design-system/pill";
import { BellPanel, useUnreadCount } from "@/features/notifications/bell-panel";
import type { Environment, Org, Project } from "@/lib/api";
import { fmtMoneyPerMonth } from "@/lib/fmt";
import { initialsOf, useSession, useSessionStore } from "@/lib/session";
import { useSetPaletteOpen, useTheme, useToggleTheme, useUIStore } from "@/lib/store";
import { cn } from "@/lib/utils";
import { Mark } from "./mark";

/**
 * The context bar: crumb elements are switchers, not links (02-ia). Org via
 * avatar (SW1), project via crumb (SW2), env via pill — env switching rewrites
 * only ?env= (ADR-013). Search is ⌘K; the assistant is chrome (⌘J, ADR-020).
 */

interface CtxProps {
  orgs: Org[];
  org: Org;
  project?: Project;
  projects?: Project[];
  environments?: Environment[];
  env?: string;
  searchPlaceholder?: string;
  showAssistant?: boolean;
}

export const menuContentClass =
  "z-50 min-w-[240px] rounded-xl border border-hair bg-surface p-1 shadow-e2";
export const menuItemClass =
  "flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-12p5 font-medium outline-none data-[highlighted]:bg-surface2";

export function Ctx({
  orgs,
  org,
  project,
  projects = [],
  environments = [],
  env,
  searchPlaceholder = "Search projects, services, docs…",
  showAssistant = false,
}: CtxProps) {
  const navigate = useNavigate();
  const session = useSession();
  const signOut = useSessionStore((s) => s.signOut);
  const theme = useTheme();
  const toggleTheme = useToggleTheme();
  const setPaletteOpen = useSetPaletteOpen();
  const [bellOpen, setBellOpen] = useState(false);
  const assistantOpen = useUIStore((s) => s.assistantOpen);
  const openAssistant = useUIStore((s) => s.openAssistant);
  const closeAssistant = useUIStore((s) => s.closeAssistant);
  const unread = useUnreadCount();

  return (
    <header className="ctx">
      <Mark />

      {/* Org switcher — SW1: the billing & policy boundary */}
      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild>
          <button type="button" className="crumb">
            <span className="cav orggrad">{org.name.slice(0, 1).toUpperCase()}</span>
            {org.name}
            <Icon id="s-chevd" className="cv h-[11px] w-[11px]" />
          </button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content className={menuContentClass} align="start" sideOffset={6}>
            <div className="eyebrow px-3 py-2">Organizations</div>
            {orgs.map((o) => (
              <DropdownMenu.Item
                key={o.id}
                className={menuItemClass}
                onSelect={() => navigate({ to: "/$org", params: { org: o.slug } })}
              >
                <span className="flex-1">
                  <span className="block">{o.name}</span>
                  <span className="text-10p5 text-ink3">
                    {o.plan === "business"
                      ? "Business · 4 projects · $482/mo"
                      : "Free · 0 projects"}
                  </span>
                </span>
                {o.id === org.id && <Icon id="s-check" className="h-3.5 w-3.5 text-steel" />}
              </DropdownMenu.Item>
            ))}
            <DropdownMenu.Separator className="my-1 h-px bg-hair" />
            <div className="px-3 py-2 text-10p5 text-ink3">
              switching lands on the org's projects home <Kbd>↵</Kbd>
            </div>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>

      {project ? (
        <>
          <span className="csep">/</span>
          {/* Project switcher — SW2: status dots and cost in the menu itself */}
          <DropdownMenu.Root>
            <DropdownMenu.Trigger asChild>
              <button type="button" className="crumb">
                <Icon id="s-hex" className="h-3 w-3 text-steel" />
                {project.name}
                <Icon id="s-chevd" className="cv h-[11px] w-[11px]" />
              </button>
            </DropdownMenu.Trigger>
            <DropdownMenu.Portal>
              <DropdownMenu.Content className={menuContentClass} align="start" sideOffset={6}>
                {projects.map((p) => (
                  <DropdownMenu.Item
                    key={p.id}
                    className={menuItemClass}
                    onSelect={() =>
                      navigate({
                        to: "/$org/$project",
                        params: { org: org.slug, project: p.name },
                        search: { env: "production" },
                      })
                    }
                  >
                    <Dot tone={healthDotTone(p.health ?? "ok")} />
                    <span className="flex-1">{p.name}</span>
                    <span className="mono text-10p5 text-ink3">
                      {fmtMoneyPerMonth(p.monthly_cost_cents ?? 0)}
                    </span>
                    {p.id === project.id && (
                      <Icon id="s-check" className="h-3.5 w-3.5 text-steel" />
                    )}
                  </DropdownMenu.Item>
                ))}
                <DropdownMenu.Separator className="my-1 h-px bg-hair" />
                <div className="px-3 py-2 text-10p5 text-ink3">
                  switching keeps your surface — overview stays overview
                </div>
              </DropdownMenu.Content>
            </DropdownMenu.Portal>
          </DropdownMenu.Root>

          {environments.length > 0 && env ? (
            <>
              <span className="csep">/</span>
              {/* Env switcher — env is a filter: rewrites ?env= only (ADR-013) */}
              <DropdownMenu.Root>
                <DropdownMenu.Trigger asChild>
                  <button type="button" className="envpill">
                    <Icon id="s-env" />
                    {env}
                    <Icon id="s-chevd" className="h-[11px] w-[11px]" />
                  </button>
                </DropdownMenu.Trigger>
                <DropdownMenu.Portal>
                  <DropdownMenu.Content className={menuContentClass} align="start" sideOffset={6}>
                    {environments.map((e) => (
                      <DropdownMenu.Item
                        key={e.id}
                        className={menuItemClass}
                        onSelect={() =>
                          navigate({
                            to: ".",
                            search: (prev: Record<string, unknown>) => ({ ...prev, env: e.name }),
                          })
                        }
                      >
                        <span className="flex-1">{e.name}</span>
                        <span className="mono text-10p5 text-ink3">
                          {fmtMoneyPerMonth(e.monthly_cost_cents ?? 0)}
                        </span>
                        {e.name === env && <Icon id="s-check" className="h-3.5 w-3.5 text-steel" />}
                      </DropdownMenu.Item>
                    ))}
                  </DropdownMenu.Content>
                </DropdownMenu.Portal>
              </DropdownMenu.Root>
            </>
          ) : null}
        </>
      ) : null}

      <button type="button" className="ctxsearch" onClick={() => setPaletteOpen(true)}>
        <Icon id="s-search" />
        <span className="flex-1 text-left">{searchPlaceholder}</span>
        <Kbd>⌘K</Kbd>
      </button>

      <span className="flex-1" />

      {showAssistant ? (
        // Assist button filled ⇔ drawer open (16-qa)
        <button
          type="button"
          className={cn("btn a h-[30px]", assistantOpen && "!bg-assist !text-canvas")}
          onClick={() => (assistantOpen ? closeAssistant() : openAssistant())}
        >
          <Icon id="s-ai" />
          Assistant
          <Kbd>⌘J</Kbd>
        </button>
      ) : null}

      <button
        type="button"
        className="icb"
        aria-label="Notifications"
        onClick={() => setBellOpen((v) => !v)}
      >
        <Icon id="s-bell" />
        {unread > 0 ? (
          <span className="absolute top-1 right-1 h-2 w-2 rounded-full bg-err" aria-hidden="true" />
        ) : null}
      </button>
      {bellOpen ? (
        <BellPanel open={bellOpen} onClose={() => setBellOpen(false)} org={org.slug} />
      ) : null}
      <a
        className="icb"
        href="https://docs.steloit.app"
        target="_blank"
        rel="noreferrer"
        aria-label="Documentation"
      >
        <Icon id="s-doc" />
      </a>

      {/* User menu — P1 */}
      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild>
          <button type="button" className="uav cursor-pointer" aria-label="Account">
            {session ? initialsOf(session.name) : "?"}
          </button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            className={cn(menuContentClass, "w-[256px]")}
            align="end"
            sideOffset={6}
          >
            <div className="flex items-center gap-2.5 border-hair border-b px-3 py-2.5">
              <span className="uav">{session ? initialsOf(session.name) : "?"}</span>
              <span>
                <span className="block text-12p5 font-semibold">{session?.name}</span>
                <span className="mono block text-10p5 text-ink3">{session?.email}</span>
              </span>
            </div>
            <DropdownMenu.Item
              className={menuItemClass}
              onSelect={() => navigate({ to: "/account/profile" })}
            >
              <Icon id="s-gear" className="h-3.5 w-3.5 text-ink3" />
              Account settings
            </DropdownMenu.Item>
            <DropdownMenu.Item
              className={menuItemClass}
              onSelect={(e) => {
                e.preventDefault();
                toggleTheme();
              }}
            >
              <Icon id="s-eye" className="h-3.5 w-3.5 text-ink3" />
              Theme · {theme === "dark" ? "Dark" : "Light"}
              <Icon id="s-chevd" className="ml-auto h-[11px] w-[11px] text-ink3" />
            </DropdownMenu.Item>
            <DropdownMenu.Item className={menuItemClass} onSelect={() => setPaletteOpen(true)}>
              <Icon id="s-term" className="h-3.5 w-3.5 text-ink3" />
              Keyboard shortcuts
              <span className="ml-auto">
                <Kbd>?</Kbd>
              </span>
            </DropdownMenu.Item>
            <DropdownMenu.Item className={menuItemClass} asChild>
              <a href="https://docs.steloit.app" target="_blank" rel="noreferrer">
                <Icon id="s-doc" className="h-3.5 w-3.5 text-ink3" />
                Documentation
              </a>
            </DropdownMenu.Item>
            <DropdownMenu.Separator className="my-1 h-px bg-hair" />
            <div className="flex items-center gap-2.5 px-3 py-2 text-12p5">
              <Dot tone="ok" />
              All systems normal
            </div>
            <DropdownMenu.Separator className="my-1 h-px bg-hair" />
            <DropdownMenu.Item
              className={cn(menuItemClass, "text-err")}
              onSelect={() => {
                signOut();
                navigate({ to: "/login" });
              }}
            >
              <Icon id="s-x" className="h-3.5 w-3.5" />
              Sign out
            </DropdownMenu.Item>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </header>
  );
}
