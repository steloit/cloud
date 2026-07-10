import { QueryClientProvider } from "@tanstack/react-query";
import {
  createRootRoute,
  ErrorComponent,
  type ErrorComponentProps,
  Outlet,
} from "@tanstack/react-router";
import { useEffect } from "react";
import { Toaster } from "sonner";
import sprite from "@/assets/sprite.svg?raw";
import "@/lib/api";
import { queryClient } from "@/lib/api/query-client";
import { useUIStore } from "@/lib/store";
import { applyTheme, DEFAULT_THEME } from "@/lib/theme";

function ThemeEffect() {
  const theme = useUIStore((s) => s.theme);
  useEffect(() => {
    applyTheme(theme ?? DEFAULT_THEME);
  }, [theme]);
  return null;
}

function RootComponent() {
  return (
    <QueryClientProvider client={queryClient}>
      {/* Single symbol sprite — icons resolve via <use href="#s-…"> */}
      {/* biome-ignore lint/security/noDangerouslySetInnerHtml: static sprite asset from 15-assets */}
      <div hidden dangerouslySetInnerHTML={{ __html: sprite }} />
      <ThemeEffect />
      <Outlet />
      <Toaster position="bottom-right" />
    </QueryClientProvider>
  );
}

function RootErrorComponent(props: ErrorComponentProps) {
  return (
    <div className="p-8">
      <h1 className="h1 text-err">Something went wrong</h1>
      <ErrorComponent error={props.error} />
    </div>
  );
}

export const Route = createRootRoute({
  component: RootComponent,
  errorComponent: RootErrorComponent,
});
