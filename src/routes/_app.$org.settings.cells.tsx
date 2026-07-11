import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useRef } from "react";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Icon } from "@/design-system/icon";
import { Dot, Pill } from "@/design-system/pill";
import { useOrgs } from "@/features/org/hooks";
import type { Cell } from "@/lib/api";
import { listCellsOptions } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * X2 · Organization · Cells (BYOC) — connect AWS/GCP/Azure as deployment
 * targets; a cell is infrastructure, not a product. Cells come from
 * listCellsOptions; frame-only strings (account/role identity, agent lines,
 * heartbeats) live in a display map. NOTE: the X2 frame says "3 environments
 * deployed here" and region aws · us-east-1 for the production cell; fixtures
 * say 2 + aws/ap-south-1 — fixtures win for env count/region, frame copy is
 * kept where static (finding).
 */

interface CellDisplay {
  identity: string;
  agentVersion: string;
  agentNote: string;
  agentTone: "ok" | "warn";
  heartbeat: string;
  primaryAction: string;
  primaryWarn?: boolean;
}

const CELL_DISPLAY: Record<string, CellDisplay> = {
  cell_prod: {
    identity: "account 4471-8820-1123 · role arn:aws:iam::447188201123:role/steloit-cell",
    agentVersion: "agent v2.14",
    agentNote: "up to date",
    agentTone: "ok",
    heartbeat: "last heartbeat 4 s ago",
    primaryAction: "Health & upgrades",
  },
  cell_eu: {
    identity: "project acme-eu-prod · WIF pool steloit-cell",
    agentVersion: "agent v2.12",
    agentNote: "v2.14 ready",
    agentTone: "warn",
    heartbeat: "last heartbeat 6 s ago",
    primaryAction: "Review upgrade",
    primaryWarn: true,
  },
};

const OPS_REASON = "Cell ops land in Phase 5";

const CONNECT_STEPS = [
  { state: "done", title: "Choose provider", sub: "AWS" },
  { state: "done", title: "Prerequisites", sub: "quotas · regions · IAM" },
  { state: "active", title: "Cross-account trust", sub: "CloudFormation role" },
  { state: "todo", title: "Provision cell", sub: "agent + networking" },
  { state: "todo", title: "Health check", sub: "heartbeat + smoke" },
] as const;

function CellCard({ cell }: { cell: Cell }) {
  const d = CELL_DISPLAY[cell.id];
  const prov = cell.provider.toUpperCase();
  const healthy = cell.status === "healthy";
  const envCount = cell.environments_deployed ?? 0;
  return (
    <Card className={cn("flex flex-col gap-2.5 p-4", healthy && "border-ok/40")}>
      <div className="flex items-center gap-2.5">
        <span className="glyph h-[26px] w-[26px]">
          <Icon id="s-globe" className="!h-[13px] !w-[13px]" />
        </span>
        <b className="text-[13px]">{cell.name}</b>
        <Pill tone="mut">customer {prov}</Pill>
        <span className="sp" />
        {cell.status === "upgrade_available" ? (
          <Pill tone="warn">agent upgrade available</Pill>
        ) : (
          <Pill tone={healthy ? "ok" : "warn"}>{cell.status.replace("_", " ")}</Pill>
        )}
      </div>
      <div className="mono text-[9.5px] text-ink3">
        {cell.region.replace("/", " · ")}
        {d ? ` · ${d.identity}` : ""}
      </div>
      <div className="flex gap-3.5 border-hair border-t pt-2.5 text-[10px] text-ink2">
        <span>
          {envCount} environment{envCount === 1 ? "" : "s"} deployed here
        </span>
        {d ? (
          <span>
            {d.agentVersion} ·{" "}
            <span className={d.agentTone === "ok" ? "text-ok" : "text-warn"}>{d.agentNote}</span>
          </span>
        ) : null}
        {d ? <span>{d.heartbeat}</span> : null}
      </div>
      <div className="flex gap-1.5">
        <Btn
          variant="s"
          className={cn("h-6 px-2 text-[10px]", d?.primaryWarn && "text-warn")}
          disabled
          disabledReason={OPS_REASON}
        >
          {d?.primaryAction ?? "Health & upgrades"}
        </Btn>
        <Btn variant="gh" className="h-6 px-2 text-[10px]" disabled disabledReason={OPS_REASON}>
          Network
        </Btn>
        <Btn variant="gh" className="h-6 px-2 text-[10px]" disabled disabledReason={OPS_REASON}>
          Support access
        </Btn>
      </div>
    </Card>
  );
}

function CellsPage() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const cells = useQuery({ ...listCellsOptions({ path: { org } }), select: (r) => r.data ?? [] });
  const connectRef = useRef<HTMLDivElement>(null);

  return (
    <>
      <SnavSettings org={org} orgName={orgRecord?.name ?? org} project="ecommerce" active="cells" />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Organization · Cells"
            sub={
              <>
                Connect AWS, GCP or Azure accounts as deployment targets — this is connecting{" "}
                <b>infrastructure</b>, not creating a product. Once healthy, a cell is just another
                choice in the region selector. Cells are a <b>Business &amp; Enterprise</b>{" "}
                capability — the control-plane fee is per cell.
              </>
            }
          >
            <Btn
              variant="p"
              onClick={() => connectRef.current?.scrollIntoView({ behavior: "smooth" })}
            >
              Connect cloud
            </Btn>
          </Pghead>

          <div className="grid grid-cols-2 gap-3">
            {(cells.data ?? []).map((c) => (
              <CellCard key={c.id} cell={c} />
            ))}
          </div>

          <div ref={connectRef}>
            <Card className="flex flex-col overflow-hidden p-0">
              <div className="flex items-center gap-2.5 border-hair border-b px-4 py-2.5">
                <b className="text-[12.5px]">Connect a new cloud</b>
                <Pill tone="st">in progress</Pill>
                <span className="sp" />
                <span className="mono text-[9.5px] text-ink3">
                  Acme APAC Cell · aws · ap-south-1
                </span>
              </div>
              <div className="flex">
                <div className="flex w-[230px] shrink-0 flex-col gap-1 border-hair border-r p-3.5">
                  {CONNECT_STEPS.map((s, i) => (
                    <div
                      key={s.title}
                      className={cn(
                        "flex items-center gap-2.5 rounded-[7px] px-2 py-1.5",
                        s.state === "active" && "bg-steel-tint",
                      )}
                    >
                      <span
                        className={cn(
                          "inline-flex h-[18px] w-[18px] items-center justify-center rounded-full text-[9px]",
                          s.state === "done" && "bg-ok text-[#0B0E11]",
                          s.state === "active" && "bg-prov text-[#0B0E11]",
                          s.state === "todo" && "border border-hair text-ink3",
                        )}
                      >
                        {s.state === "done" ? "✓" : s.state === "active" ? "●" : i + 1}
                      </span>
                      <div>
                        <div className="text-[11px] text-ink1">{s.title}</div>
                        <div className="text-[8.5px] text-ink3">{s.sub}</div>
                      </div>
                    </div>
                  ))}
                </div>
                <div className="flex flex-1 flex-col gap-2.5 p-4">
                  <div>
                    <b className="text-[12.5px]">Cross-account trust</b>
                    <div className="mt-1 text-[10.5px] leading-relaxed text-ink3">
                      Run one CloudFormation stack — it creates a scoped role Steloit assumes. No
                      static keys ever leave your account; trust is an external-ID-guarded
                      assume-role.
                    </div>
                  </div>
                  <Card className="flex flex-col gap-1.5 bg-surface2 p-3">
                    <Eyebrow>Prerequisites — validated before anything provisions</Eyebrow>
                    <div className="flex items-start gap-2 text-[10.5px] text-ink2">
                      <Dot tone="ok" />
                      <span>
                        Region ap-south-1 enabled · service quotas sufficient (vCPU 64/128)
                      </span>
                    </div>
                    <div className="flex items-start gap-2 text-[10.5px] text-ink2">
                      <Dot tone="ok" />
                      <span>No SCP blocks on required services (EKS, RDS, S3, VPC)</span>
                    </div>
                    <div className="flex items-start gap-2 text-[10.5px] text-ink2">
                      <Dot tone="warn" />
                      <span>
                        Dedicated VPC recommended — we can create one, or attach yours in the
                        Network step
                      </span>
                    </div>
                  </Card>
                  <Copybit>
                    aws cloudformation deploy --template-url steloit-cell.yaml --parameter
                    ExternalId=ex_9f2a
                  </Copybit>
                  <div className="flex items-center gap-2.5">
                    <span className="mono text-[9.5px] text-ink3">
                      waiting for stack CREATE_COMPLETE — auto-detected via the trust role
                    </span>
                    <span className="sp" />
                    <Btn
                      variant="s"
                      disabled
                      disabledReason="Cell connect (POST /orgs/{org}/cells) lands in Phase 5"
                    >
                      I've run it →
                    </Btn>
                  </div>
                </div>
              </div>
            </Card>
          </div>

          <p className="text-[10px] leading-relaxed text-ink3">
            A cell is <b>Infrastructure</b>, governed by existing primitives: an{" "}
            <span className="mono">allowed-regions</span> Policy (G7) can permit or forbid deploying
            production into a customer cell exactly as it gates a managed region — no BYOC-specific
            policy engine.
          </p>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/settings/cells")({
  component: CellsPage,
});
