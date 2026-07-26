import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Flabel, Inp } from "@/design-system/inp";
import { AuthShell, AuthTitle } from "@/features/auth/auth-shell";

/**
 * A3 · Forgot password — form + sent state. The sent message is identical
 * whether or not the account exists — no account enumeration.
 * Route not in 02-ia's URL map (finding): minimal addition for A3.
 */
function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);

  return (
    <AuthShell>
      <AuthTitle
        title="Reset your password"
        sub="Enter your account email and we'll send a reset link."
      />
      <form
        className="flex flex-col gap-3.5"
        onSubmit={(e) => {
          e.preventDefault();
          setSent(true);
        }}
      >
        <div>
          <Flabel htmlFor="email">Email</Flabel>
          <Inp
            id="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
            required
          />
        </div>
        <Btn variant="p" type="submit" className="w-full justify-center">
          Send reset link
        </Btn>
      </form>
      {sent ? (
        <Card className="border-ok p-3.5 text-12 leading-relaxed text-ink2">
          If an account exists for {email || "that address"}, a reset link is on its way. It expires
          in 30 minutes.
        </Card>
      ) : null}
      <div className="text-center text-11p5">
        <Link to="/login" className="font-medium text-steel">
          ← Back to sign in
        </Link>
      </div>
    </AuthShell>
  );
}

export const Route = createFileRoute("/forgot-password")({
  component: ForgotPasswordPage,
});
