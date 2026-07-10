import type { ReactNode } from "react";
import { Icon } from "@/design-system/icon";

/**
 * Auth split layout (frames A1–A4): fixed-dark brand panel — dark in both
 * themes — plus the centered form card. Tier-3 identity chrome: brand mark
 * only, no tenancy UI.
 */
export function AuthShell({ children }: { children: ReactNode }) {
  return (
    <div className="authwrap min-h-screen">
      <aside className="authbrand max-lg:hidden">
        <div className="flex items-center gap-2.5">
          <Icon id="s-hex" className="h-[26px] w-[26px] text-steel" />
          <span className="text-[15px] font-semibold text-[#E6EAEE]">Steloit</span>
        </div>
        <div>
          <h2 className="text-[30px] font-semibold leading-[1.2] tracking-[-0.5px] text-[#E6EAEE]">
            One project.
            <br />
            One platform.
            <br />
            Infinite possibilities.
          </h2>
          <p className="mt-4 max-w-[380px] text-[13px] leading-relaxed text-[#9AA6B2]">
            Postgres, cache, storage, queues and compute — provisioned in minutes, monitored from
            the first second, priced before they exist.
          </p>
        </div>
        <div className="mono text-[11px] text-[#5C6773]">
          $ steloit project create · time-to-first-connection &lt; 5 min
        </div>
      </aside>
      <div className="authform">
        <div className="acard">{children}</div>
      </div>
    </div>
  );
}

export function AuthTitle({ title, sub }: { title: string; sub: ReactNode }) {
  return (
    <div>
      <h1 className="h1">{title}</h1>
      <div className="hsub">{sub}</div>
    </div>
  );
}
