import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Icon } from "@/design-system/icon";
import { Flabel, Inp } from "@/design-system/inp";
import { AuthShell, AuthTitle } from "@/features/auth/auth-shell";
import { cn } from "@/lib/utils";

/**
 * A4 · Reset password. Requirements checklist + expiring link; resets sign
 * out all other sessions. Route not in 02-ia's URL map (finding).
 */
function ResetPasswordPage() {
  const navigate = useNavigate();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string>();

  const checks = [
    { label: "At least 12 characters", pass: password.length >= 12 },
    {
      label: "Contains a number and a symbol",
      pass: /\d/.test(password) && /[^a-zA-Z0-9]/.test(password),
    },
    { label: "Not a previously used password", pass: password.length > 0 },
  ];
  const strength = checks.filter((c) => c.pass).length + (password.length >= 16 ? 1 : 0);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (checks.some((c) => !c.pass)) return setError("Meet every requirement below.");
    if (password !== confirm) return setError("Passwords don't match.");
    toast.success("Password reset. Sign in with your new password.");
    navigate({ to: "/login" });
  };

  return (
    <AuthShell>
      <AuthTitle
        title="Set a new password"
        sub={
          <>
            for priya@acme.dev · <span className="mono">link expires in 27:12</span>
          </>
        }
      />
      <form className="flex flex-col gap-3.5" onSubmit={submit}>
        <div>
          <Flabel htmlFor="password">New password</Flabel>
          <Inp
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="new-password"
          />
          <div className="mt-2 flex gap-1.5" aria-hidden="true">
            {[0, 1, 2, 3].map((i) => (
              <span
                key={i}
                className={cn("h-1 flex-1 rounded-full", i < strength ? "bg-ok" : "bg-surface2")}
              />
            ))}
          </div>
        </div>
        <div>
          <Flabel htmlFor="confirm">Confirm password</Flabel>
          <Inp
            id="confirm"
            type="password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            autoComplete="new-password"
            error={error}
          />
        </div>
        <ul className="flex flex-col gap-1 text-11p5">
          {checks.map((c) => (
            <li
              key={c.label}
              className={cn("flex items-center gap-1.5", c.pass ? "text-ok" : "text-ink3")}
            >
              <Icon id={c.pass ? "s-check" : "s-x"} className="h-[11px] w-[11px]" /> {c.label}
            </li>
          ))}
        </ul>
        <Btn variant="p" type="submit" className="w-full justify-center">
          Reset password
        </Btn>
        <div className="text-center text-11 text-ink3">
          You'll be signed out of all other sessions.
        </div>
      </form>
    </AuthShell>
  );
}

export const Route = createFileRoute("/reset-password")({
  component: ResetPasswordPage,
});
