import { redirect } from "@tanstack/react-router";
import { decide, sanitizeReturnTo, useSessionStore } from "@/lib/session";

/** beforeLoad adapters over the pure guard logic in lib/session. */

export function requireSession(currentPath: string): void {
  const authed = useSessionStore.getState().session !== null;
  const target = decide({ authed, routeKind: "app" });
  if (target === "/login") {
    throw redirect({ to: "/login", search: { returnTo: currentPath } });
  }
}

export function bounceAuthed(): void {
  const authed = useSessionStore.getState().session !== null;
  if (decide({ authed, routeKind: "auth" }) === "/") {
    throw redirect({ to: "/" });
  }
}

export function resolveReturnTo(candidate: string | undefined): string {
  return sanitizeReturnTo(candidate) ?? "/";
}
