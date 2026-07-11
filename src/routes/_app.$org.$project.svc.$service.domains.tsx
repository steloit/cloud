import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Inp } from "@/design-system/inp";
import { Stlab } from "@/design-system/pill";
import { AddDomainDrawer } from "@/features/services/domain-drawer";
import { useServices } from "@/features/services/hooks";
import { type Domain, listDomainsOptions } from "@/lib/api";

/**
 * D21 · Web · Domains — rows come from GET /services/{svc}/domains; the
 * frame-fixed display strings (type, TLS renewal) are a per-domain map merged
 * with the live data. Web only — workers never terminate HTTP.
 */

interface DomainDisplay {
  type: string;
  tls: string;
}

const DISPLAY: Record<string, DomainDisplay> = {
  "api.acme-store.com": { type: "custom · primary", tls: "valid · renews in 61 d" },
};

function displayFor(dom: Domain): DomainDisplay {
  return DISPLAY[dom.domain] ?? { type: "custom", tls: dom.status === "issued" ? "valid" : "—" };
}

function DomainsPage() {
  const { project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(env);
  const svc = services.data?.find((s) => s.name === service || s.id === service);
  const domains = useQuery({
    ...listDomainsOptions({ path: { service: svc?.id ?? "" } }),
    enabled: !!svc,
    select: (r) => r.data ?? [],
  });
  const [domainInput, setDomainInput] = useState("checkout.acme-store.com");
  const [drawerOpen, setDrawerOpen] = useState(false);

  if (!svc) return <main className="main" />;
  if (svc.product !== "web") {
    return (
      <main className="main">
        <div className="pgpad">
          <h1 className="h1">Domains</h1>
          <p className="hsub">
            Domains (D21) is a web surface — {svc.name} is {svc.product}.
          </p>
        </div>
      </main>
    );
  }

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Domains"
          sub={
            <span className="mono">
              {svc.name} · {env} · certificates issued and renewed automatically
            </span>
          }
        />

        <div className="tblwrap">
          <table className="tbl">
            <thead>
              <tr>
                <th>Domain</th>
                <th>Type</th>
                <th>TLS</th>
                <th>Status</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {(domains.data ?? []).map((dom) => {
                const display = displayFor(dom);
                return (
                  <tr key={dom.id}>
                    <td className="mono">{dom.domain}</td>
                    <td>{display.type}</td>
                    <td>
                      <Stlab tone="ok">{display.tls}</Stlab>
                    </td>
                    <td>
                      <Stlab tone="ok">serving</Stlab>
                    </td>
                    <td className="text-right">
                      <Btn variant="gh" disabled disabledReason="already primary">
                        Make primary
                      </Btn>
                    </td>
                  </tr>
                );
              })}
              {/* Platform row is frame-fixed (D21) — the always-on fallback
                  subdomain isn't a Domain resource in 08-api. */}
              <tr>
                <td className="mono">api.internal.steloit.app</td>
                <td>platform · always on</td>
                <td>
                  <Stlab tone="ok">valid</Stlab>
                </td>
                <td>
                  <Stlab tone="ok">serving</Stlab>
                </td>
                <td className="mono text-right text-[11px] text-ink3">fallback</td>
              </tr>
            </tbody>
          </table>
        </div>

        <Card className="flex flex-col gap-2.5 p-4">
          <div className="text-[12px] font-semibold">Add domain</div>
          <div className="flex items-center gap-2.5">
            <Inp
              className="mono max-w-64 flex-1"
              value={domainInput}
              onChange={(e) => setDomainInput(e.target.value)}
              aria-label="Domain to add"
            />
            <Copybit>CNAME → api.internal.steloit.app</Copybit>
            <Btn variant="p" onClick={() => setDrawerOpen(true)}>
              Verify &amp; issue
            </Btn>
          </div>
        </Card>

        <p className="text-[11px] leading-relaxed text-ink3">
          Verification polls DNS and issues the certificate on first success — no cert files, no
          renewal cron, nothing to forget. Previews get generated subdomains under
          *.preview.steloit.app automatically.
        </p>

        {drawerOpen ? (
          <AddDomainDrawer
            svc={svc}
            project={project}
            env={env}
            domain={domainInput}
            onClose={() => setDrawerOpen(false)}
          />
        ) : null}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/domains")({
  component: DomainsPage,
});
