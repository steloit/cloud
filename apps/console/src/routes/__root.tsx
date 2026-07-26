import { QueryClientProvider } from "@tanstack/react-query";
import { createRootRoute, type ErrorComponentProps, Outlet } from "@tanstack/react-router";
import { useEffect } from "react";
import { Toaster } from "sonner";
import sprite from "@/assets/sprite.svg?raw";
import "@/lib/api";
import { NotFoundPage, ServerErrorPage } from "@/features/errors/error-pages";
import { NetworkLossBanner } from "@/features/errors/failure-states";
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
      <NetworkLossBanner />
      <Outlet />
      <Toaster position="bottom-right" />
    </QueryClientProvider>
  );
}

function RootErrorComponent(props: ErrorComponentProps) {
  return <ServerErrorPage error={props.error} />;
}

export const Route = createRootRoute({
  component: RootComponent,
  errorComponent: RootErrorComponent,
  notFoundComponent: NotFoundPage,
});
