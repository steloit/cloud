import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { Btn } from "@/design-system/btn";
import { Icon } from "@/design-system/icon";
import { Flabel, Inp } from "@/design-system/inp";
import { AuthShell, AuthTitle } from "@/features/auth/auth-shell";
import { bounceAuthed, resolveReturnTo } from "@/features/auth/guards";
import { useSessionStore } from "@/lib/session";

/** A1 · Sign in. Session AuthN sits outside the REST spec (finding) — the
 * canon-mode form establishes a local session. */
function LoginPage() {
  const navigate = useNavigate();
  const { returnTo } = Route.useSearch();
  const signIn = useSessionStore((s) => s.signIn);
  const [email, setEmail] = useState("priya@acme.dev");
  const [password, setPassword] = useState("••••••••••••");
  const [error, setError] = useState<string>();

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!email.includes("@")) {
      setError("Enter the email you signed up with.");
      return;
    }
    if (password.length < 12) {
      setError("Passwords are at least 12 characters.");
      return;
    }
    // Canon members carry their fixture roles — signing in as marco@acme.dev
    // (developer) makes the E3 permission grammar reachable for real.
    const role = email === "marco@acme.dev" || email === "asha@acme.dev" ? "developer" : "admin";
    signIn({
      email,
      name: email === "priya@acme.dev" ? "Priya Sharma" : localPartName(email),
      role,
    });
    navigate({ to: resolveReturnTo(returnTo) });
  };

  return (
    <AuthShell>
      <AuthTitle title="Sign in" sub="Welcome back to Steloit" />
      <button type="button" className="sso">
        <Icon id="s-org" className="h-4 w-4" />
        Continue with GitHub
      </button>
      <button type="button" className="sso">
        Continue with Google
      </button>
      <div className="ordiv">OR</div>
      <form className="flex flex-col gap-3.5" onSubmit={submit}>
        <div>
          <Flabel htmlFor="email">Email</Flabel>
          <Inp
            id="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
          />
        </div>
        <div>
          <Flabel
            htmlFor="password"
            aside={
              <Link to="/forgot-password" className="text-11 font-medium text-steel">
                Forgot?
              </Link>
            }
          >
            Password
          </Flabel>
          <Inp
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            error={error}
          />
        </div>
        <Btn variant="p" type="submit" className="w-full justify-center">
          Sign in
        </Btn>
      </form>
      <div className="text-center text-11p5 text-ink3">
        New to Steloit?{" "}
        <Link to="/signup" className="font-medium text-steel">
          Create an account
        </Link>{" "}
        · <span className="font-medium text-steel">Use SSO</span>
      </div>
    </AuthShell>
  );
}

function localPartName(email: string): string {
  const local = email.split("@")[0] ?? "there";
  return local.charAt(0).toUpperCase() + local.slice(1);
}

export const Route = createFileRoute("/login")({
  beforeLoad: () => bounceAuthed(),
  validateSearch: (search: Record<string, unknown>): { returnTo?: string } => ({
    returnTo: typeof search.returnTo === "string" ? search.returnTo : undefined,
  }),
  component: LoginPage,
});
