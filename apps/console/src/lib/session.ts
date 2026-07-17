import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { Role } from "./rbac.ts";

/**
 * Session AuthN is outside the REST spec (14-development/architecture.md:
 * "AuthN: session + tokens"; 08-api has no /auth endpoint — surfaced as a
 * finding). In canon mode the sign-in form establishes a local session for
 * the canon user; a real auth service slots in behind signIn() later.
 */
export interface Session {
  email: string;
  name: string;
  role: Role;
}

interface SessionState {
  session: Session | null;
  signIn: (session: Session) => void;
  signOut: () => void;
}

export const useSessionStore = create<SessionState>()(
  persist(
    (set) => ({
      session: null,
      signIn: (session) => set({ session }),
      signOut: () => set({ session: null }),
    }),
    { name: "steloit-session" },
  ),
);

export const useSession = () => useSessionStore((s) => s.session);

/** Initials for the .uav avatar — "Priya Sharma" → "PS". */
export function initialsOf(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => (part[0] ?? "").toUpperCase())
    .join("");
}

// ---------------------------------------------------------------------------
// Pure route-guard logic (unit-testable; adapters live in routes' beforeLoad).
// ---------------------------------------------------------------------------

export type RouteKind = "app" | "auth" | "onboarding";

/** Where to send the user, or null to let the route render. */
export function decide(input: { authed: boolean; routeKind: RouteKind }): "/login" | "/" | null {
  if (!input.authed && input.routeKind !== "auth") return "/login";
  if (input.authed && input.routeKind === "auth") return "/";
  return null;
}

/** Same-origin relative paths only; never bounce back into auth/onboarding. */
export function sanitizeReturnTo(candidate: string | undefined): string | null {
  if (!candidate) return null;
  if (!candidate.startsWith("/") || candidate.startsWith("//")) return null;
  if (candidate.startsWith("/login") || candidate.startsWith("/signup")) return null;
  if (candidate.startsWith("/onboarding")) return null;
  return candidate;
}
