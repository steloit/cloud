import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { Btn } from "@/design-system/btn";
import { Icon } from "@/design-system/icon";
import { Flabel, Inp } from "@/design-system/inp";
import { AuthShell, AuthTitle } from "@/features/auth/auth-shell";
import { bounceAuthed } from "@/features/auth/guards";
import { useSessionStore } from "@/lib/session";
import { cn } from "@/lib/utils";

/** A2 · Sign up — free tier, no card. */
function SignupPage() {
  const navigate = useNavigate();
  const signIn = useSessionStore((s) => s.signIn);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string>();

  const strength = passwordStrength(password);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return setError("Enter your name.");
    if (!email.includes("@")) return setError("Enter a work email.");
    if (strength < 3) return setError("Passwords need 12+ characters, a number and a symbol.");
    signIn({ email, name: name.trim(), role: "owner" });
    navigate({ to: "/onboarding/org" });
  };

  return (
    <AuthShell>
      <AuthTitle title="Create your account" sub="Free tier included · no credit card required" />
      <button type="button" className="sso">
        <Icon id="s-org" className="h-4 w-4" />
        Sign up with GitHub
      </button>
      <div className="ordiv">OR</div>
      <form className="flex flex-col gap-3.5" onSubmit={submit}>
        <div>
          <Flabel htmlFor="name">Name</Flabel>
          <Inp
            id="name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoComplete="name"
          />
        </div>
        <div>
          <Flabel htmlFor="email">Work email</Flabel>
          <Inp
            id="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
          />
        </div>
        <div>
          <Flabel htmlFor="password">Password</Flabel>
          <Inp
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="new-password"
            error={error}
          />
          <div className="mt-2 flex gap-1.5" aria-hidden="true">
            {[0, 1, 2, 3].map((i) => (
              <span
                key={i}
                className={cn("h-1 flex-1 rounded-full", i < strength ? "bg-ok" : "bg-surface2")}
              />
            ))}
          </div>
          {strength >= 3 ? (
            <div className="mt-1.5 text-[11px] text-ok">
              Strong · 12+ characters ✓ · number ✓ · symbol ✓
            </div>
          ) : null}
        </div>
        <Btn variant="p" type="submit" className="w-full justify-center">
          Create account
        </Btn>
      </form>
      <div className="text-center text-[11.5px] leading-relaxed text-ink3">
        By continuing you agree to the <span className="font-medium text-steel">Terms</span> and{" "}
        <span className="font-medium text-steel">Privacy Policy</span>. Already have an account?{" "}
        <Link to="/login" className="font-medium text-steel">
          Sign in
        </Link>
      </div>
    </AuthShell>
  );
}

function passwordStrength(password: string): number {
  let score = 0;
  if (password.length >= 12) score++;
  if (/\d/.test(password)) score++;
  if (/[^a-zA-Z0-9]/.test(password)) score++;
  if (password.length >= 16) score++;
  return score;
}

export const Route = createFileRoute("/signup")({
  beforeLoad: () => bounceAuthed(),
  component: SignupPage,
});
