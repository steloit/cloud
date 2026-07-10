export type Theme = "dark" | "light";

export const DEFAULT_THEME: Theme = "dark";
export const THEME_STORAGE_KEY = "steloit-ui";

/** Theme is a class swap on <html>: `:root` is dark, `.light` flips the tokens. */
export function applyTheme(theme: Theme): void {
  document.documentElement.classList.toggle("light", theme === "light");
}
