import { createFileRoute, Outlet } from "@tanstack/react-router";
import { useEffect } from "react";
import { CommandPalette } from "@/app/command-palette";
import { requireSession } from "@/features/auth/guards";
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
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [setPaletteOpen]);

  return (
    <>
      <Outlet />
      <CommandPalette />
    </>
  );
}

export const Route = createFileRoute("/_app")({
  beforeLoad: ({ location }) => requireSession(location.href),
  component: AppLayout,
});
