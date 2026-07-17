import { createFileRoute, Outlet } from "@tanstack/react-router";
import { useEffect } from "react";
import { CommandPalette } from "@/app/command-palette";
import { AssistantDrawer } from "@/features/assistant/assistant-drawer";
import { Coachmark } from "@/features/assistant/coachmark";
import { requireSession } from "@/features/auth/guards";
import { WelcomeWidget } from "@/features/onboarding/welcome-widget";
import { useUIStore } from "@/lib/store";

/** Pathless authed group: session guard + global shortcuts (⌘K, ADR-020). */
function AppLayout() {
  const setPaletteOpen = useUIStore((s) => s.setPaletteOpen);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPaletteOpen(true);
      }
      // ⌘J — the assistant is chrome, not a FAB (ADR-020)
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "j") {
        e.preventDefault();
        const { assistantOpen, openAssistant, closeAssistant } = useUIStore.getState();
        if (assistantOpen) closeAssistant();
        else openAssistant();
      }
      // ? — the shortcut the user menu advertises; opens the palette.
      if (e.key === "?" && !e.metaKey && !e.ctrlKey && !e.altKey) {
        const t = e.target as HTMLElement | null;
        const typing =
          t instanceof HTMLInputElement || t instanceof HTMLTextAreaElement || t?.isContentEditable;
        if (!typing) {
          e.preventDefault();
          setPaletteOpen(true);
        }
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [setPaletteOpen]);

  return (
    <>
      <Outlet />
      <CommandPalette />
      <AssistantDrawer />
      <Coachmark />
      <WelcomeWidget />
    </>
  );
}

export const Route = createFileRoute("/_app")({
  beforeLoad: ({ location }) => requireSession(location.href),
  component: AppLayout,
});
