import { create } from "zustand";
import { persist } from "zustand/middleware";
import { applyTheme, DEFAULT_THEME, THEME_STORAGE_KEY, type Theme } from "./theme.ts";

/**
 * The thin global store (10-state-management/state.md): theme, command palette,
 * assistant drawer. Session lives in ./session.ts; active context {org,project,env}
 * lives in the URL (URL-as-state, ADR-009).
 */
interface UIState {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  toggleTheme: () => void;

  paletteOpen: boolean;
  setPaletteOpen: (open: boolean) => void;

  assistantOpen: boolean;
  setAssistantOpen: (open: boolean) => void;

  /** AI9: an insight id seeds the drawer with that recommendation's evidence. */
  attachedInsight: string | null;
  openAssistant: (insight?: string) => void;
  closeAssistant: () => void;
}

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      theme: DEFAULT_THEME,
      setTheme: (theme) => set({ theme }),
      toggleTheme: () => set((s) => ({ theme: s.theme === "dark" ? "light" : "dark" })),

      paletteOpen: false,
      setPaletteOpen: (open) => set({ paletteOpen: open }),

      assistantOpen: false,
      setAssistantOpen: (open) => set({ assistantOpen: open }),

      attachedInsight: null,
      openAssistant: (insight) => set({ assistantOpen: true, attachedInsight: insight ?? null }),
      closeAssistant: () => set({ assistantOpen: false, attachedInsight: null }),
    }),
    {
      name: THEME_STORAGE_KEY,
      partialize: (s) => ({ theme: s.theme }),
    },
  ),
);

useUIStore.subscribe((s) => applyTheme(s.theme));

export const useTheme = () => useUIStore((s) => s.theme);
export const useToggleTheme = () => useUIStore((s) => s.toggleTheme);
export const usePaletteOpen = () => useUIStore((s) => s.paletteOpen);
export const useSetPaletteOpen = () => useUIStore((s) => s.setPaletteOpen);
