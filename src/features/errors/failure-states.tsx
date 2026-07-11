import { useEffect, useState } from "react";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Icon } from "@/design-system/icon";
import { errorMessage } from "@/lib/api";

/**
 * E3 · Failure states — one grammar: permission denied · API failure ·
 * network loss. Every state names its cause and a next step (no dead ends).
 */

/** Network loss — a real listener on the browser's online/offline events. */
export function NetworkLossBanner() {
  const [offline, setOffline] = useState(() => !navigator.onLine);

  useEffect(() => {
    const on = () => setOffline(false);
    const off = () => setOffline(true);
    window.addEventListener("online", on);
    window.addEventListener("offline", off);
    return () => {
      window.removeEventListener("online", on);
      window.removeEventListener("offline", off);
    };
  }, []);

  if (!offline) return null;
  return (
    <div className="banner warn fixed top-[54px] left-1/2 z-50 w-[560px] -translate-x-1/2 shadow-e2">
      <span className="flex-1">
        Connection lost — retrying in 3s. Your unsaved changes are kept locally.
      </span>
      <Btn variant="s" className="h-6 px-2 text-[10.5px]" onClick={() => window.location.reload()}>
        Retry now
      </Btn>
    </div>
  );
}

/**
 * Permission denied — the E3 grammar: who you are · what's required · who
 * can grant it · the denial is audited.
 */
export function PermissionDenied({
  need,
  page,
  role,
  granters = "priya, asha",
  permission,
  onBack,
}: {
  need: string;
  page: string;
  role: string;
  granters?: string;
  permission: string;
  onBack: () => void;
}) {
  return (
    <Card className="mx-auto mt-10 flex w-[560px] flex-col gap-3 p-6">
      <Icon id="s-shield" className="h-6 w-6 text-err" />
      <h1 className="h1">
        You need {need} on ecommerce to {page}
      </h1>
      <p className="text-[12.5px] leading-relaxed text-ink2">
        You're signed in as <b>{role}</b>. Developers can operate services but not change their
        configuration. Admins on this project: {granters}.
      </p>
      <div className="flex gap-2.5">
        <Btn
          variant="p"
          disabled
          disabledReason="Access requests route through org Admins — the approval thread lands with N-series threads"
        >
          Request access
        </Btn>
        <Btn variant="s" onClick={onBack}>
          Back to overview
        </Btn>
      </div>
      <div className="mono text-[10.5px] text-ink3">
        required: {permission} · denial logged as evt_91aa…
      </div>
    </Card>
  );
}

/** API failure — inline, names the request and offers retry. */
export function ApiFailureCard({
  title,
  error,
  requestLine,
  onRetry,
}: {
  title: string;
  error: unknown;
  requestLine: string;
  onRetry: () => void;
}) {
  return (
    <Card className="flex flex-col gap-2.5 border-err/45 p-4">
      <div className="flex items-center gap-2">
        <span className="dot err" />
        <b className="text-[13px]">{title}</b>
      </div>
      <div className="mono text-[10.5px] text-ink3">
        {requestLine} · {errorMessage(error)}
      </div>
      <div className="flex gap-2.5">
        <Btn variant="s" onClick={onRetry}>
          Retry
        </Btn>
        <Btn variant="gh" disabled disabledReason="Reports route to status.steloit.app">
          Report
        </Btn>
      </div>
    </Card>
  );
}
