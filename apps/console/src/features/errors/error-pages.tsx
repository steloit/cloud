import { Link } from "@tanstack/react-router";
import { Btn } from "@/design-system/btn";
import { Copybit } from "@/design-system/copybit";
import { Dot, Pill } from "@/design-system/pill";

/**
 * E2 · Error pages — 404 and 500. The orchestrator wires these into the
 * router (notFoundComponent / errorComponent).
 */

function ErrorFrame({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-screen flex-col items-center justify-center gap-4 bg-canvas px-6 text-center">
      {children}
    </div>
  );
}

export function NotFoundPage() {
  return (
    <ErrorFrame>
      <Pill tone="mut">404 · not found</Pill>
      <div className="mono text-[64px] font-semibold leading-none tracking-tight text-ink3">
        404
      </div>
      <div className="text-14 font-semibold">This page doesn't exist in any environment.</div>
      <p className="max-w-[420px] text-12 leading-relaxed text-ink3">
        The resource may have been renamed or retired. Check the URL — or jump anywhere with ⌘K.
      </p>
      <div className="flex gap-2">
        <Link to="/">
          <Btn variant="p">Go to console</Btn>
        </Link>
        <a href="https://docs.steloit.app" target="_blank" rel="noreferrer">
          <Btn variant="s">Open docs</Btn>
        </a>
      </div>
    </ErrorFrame>
  );
}

export function ServerErrorPage({ error }: { error?: Error }) {
  return (
    <ErrorFrame>
      <Pill tone="err">500 · console error</Pill>
      <div className="mono text-[64px] font-semibold leading-none tracking-tight text-err">500</div>
      <div className="text-14 font-semibold">Something broke on our side.</div>
      <p className="max-w-[440px] text-12 leading-relaxed text-ink3">
        Your services are unaffected — this is a console error, and we've been paged. Include the
        incident ID if you contact support.
      </p>
      {error?.message ? (
        <p className="mono max-w-[440px] text-10p5 text-ink3">{error.message}</p>
      ) : null}
      <Copybit>inc_20260706_8823</Copybit>
      <div className="flex items-center gap-3">
        <Btn variant="s" onClick={() => window.location.reload()}>
          Retry
        </Btn>
        <span className="flex items-center gap-2 text-11 text-ink3">
          <Dot tone="warn" />
          status.steloit.app — investigating
        </span>
      </div>
    </ErrorFrame>
  );
}
