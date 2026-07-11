import { useMutation } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { SnavOrg } from "@/app/shell/snav-org";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Icon } from "@/design-system/icon";
import { Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { useBillingOverview, useOrgs } from "@/features/org/hooks";
import { useCreateProject, useProjects } from "@/features/projects/hooks";
import { createEstimateMutation, type Estimate, errorMessage } from "@/lib/api";
import { fmtMoney } from "@/lib/fmt";
import { cn } from "@/lib/utils";

/**
 * W2 · Create project — AI recommends with reasons, cost shown before
 * anything exists. Route is a minimal addition (02-ia lacks a create-project
 * URL; the Home snav's "New project" needs a destination — finding).
 */

/** The recommended-service rows (W2, verbatim), keyed to estimate line names. */
const RECOMMENDED = [
  {
    key: "db-main",
    lines: ["db-main"],
    name: "PostgreSQL",
    sub: "db-main · 2 vCPU / 4 GB",
    reason: "Catalog, users, orders — relational core. Backups & PITR on by default.",
  },
  {
    key: "cache",
    lines: ["cache"],
    name: "Valkey",
    sub: "cache · 1 GB",
    reason: "You described session carts — cache-mode defaults, LRU eviction.",
  },
  {
    key: "assets",
    lines: ["assets"],
    name: "Object Storage",
    sub: "assets · S3-compatible",
    reason: "Product image uploads. Signed URLs, lifecycle rules, CDN-ready.",
  },
  {
    key: "jobs",
    lines: ["jobs"],
    name: "Queue",
    sub: "jobs · at-least-once, DLQ",
    reason: "Order processing and receipts — background jobs with retries.",
  },
  {
    key: "compute",
    lines: ["api", "worker"],
    name: "Web service + Worker",
    sub: "api, worker · from your repo",
    reason: "Deploy from Git. You can also keep compute elsewhere — bindings work externally.",
  },
];

function NewProjectPage() {
  const { org } = Route.useParams();
  const navigate = useNavigate();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const projects = useProjects(org);
  const billing = useBillingOverview(org);
  const createProject = useCreateProject(org);
  const estimate = useMutation(createEstimateMutation());

  const [name, setName] = useState("ecommerce");
  const [describe, setDescribe] = useState(
    "An online store: product catalog, user sessions and carts, image uploads, order processing with background jobs and email receipts.",
  );
  const [enabled, setEnabled] = useState<Record<string, boolean>>(
    Object.fromEntries(RECOMMENDED.map((r) => [r.key, true])),
  );
  const [est, setEst] = useState<Estimate>();

  // Estimate-before-provision: nothing exists — and nothing bills — until confirmed.
  useEffect(() => {
    estimate.mutate({ body: { services: [] } }, { onSuccess: (data) => setEst(data as Estimate) });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [estimate.mutate]);

  const lineCents = new Map((est?.lines ?? []).map((l) => [l.name, l.monthly_cents ?? 0]));
  const rowCents = (row: (typeof RECOMMENDED)[number]) =>
    row.lines.reduce((sum, l) => sum + (lineCents.get(l) ?? 0), 0);
  const totalCents = RECOMMENDED.filter((r) => enabled[r.key]).reduce(
    (sum, r) => sum + rowCents(r),
    0,
  );

  const submit = () => {
    createProject.mutate(
      { path: { org }, body: { name } },
      {
        onSuccess: () => {
          toast.success(`Project ${name} created`);
          navigate({ to: "/$org", params: { org } });
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    <>
      <SnavOrg
        org={org}
        orgName={orgRecord?.name ?? org}
        projects={projects.data ?? []}
        billing={billing.data}
        active="new-project"
      />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Create a project"
            sub="A project is your application — services, environments, and cost roll up to it."
          />
          <div className="flex gap-4">
            <div className="flex max-w-[780px] flex-1 flex-col gap-3.5">
              <Card className="flex flex-col gap-2.5 p-4">
                <Eyebrow>Project name</Eyebrow>
                <div className="flex items-center gap-2.5">
                  <Inp
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    className="foc flex-1"
                  />
                  <span className="envpill">
                    <Icon id="s-env" />
                    aws · ap-south-1 (Mumbai)
                    <Icon id="s-chevd" className="h-[11px] w-[11px]" />
                  </span>
                </div>
                <div className="mono text-[10.5px] text-ink3">
                  {org}/{name || "…"} · a production environment is created by default
                </div>
              </Card>

              <Card className="flex flex-col gap-2.5 p-4">
                <Eyebrow>
                  Describe your app{" "}
                  <span className="font-normal normal-case tracking-normal">
                    — optional, or pick services yourself below
                  </span>
                </Eyebrow>
                <textarea
                  className="inp h-auto min-h-[64px] w-full resize-none py-2.5 leading-relaxed"
                  value={describe}
                  onChange={(e) => setDescribe(e.target.value)}
                />
              </Card>

              <Card className="flex flex-col p-4">
                <div className="mb-2 flex items-center gap-2.5">
                  <Eyebrow>Recommended services</Eyebrow>
                  <Pill tone="ai">assistant · you can edit everything</Pill>
                </div>
                {RECOMMENDED.map((row) => (
                  <div
                    key={row.key}
                    className="flex items-center gap-3 border-hair border-b py-2.5 last:border-b-0"
                  >
                    <button
                      type="button"
                      role="switch"
                      aria-checked={enabled[row.key]}
                      aria-label={`${row.name} — toggle`}
                      onClick={() => setEnabled((prev) => ({ ...prev, [row.key]: !prev[row.key] }))}
                      className={cn(
                        "relative h-[18px] w-8 shrink-0 rounded-full transition-colors",
                        enabled[row.key] ? "bg-steel" : "bg-surface2",
                      )}
                    >
                      <span
                        className={cn(
                          "absolute top-[2px] h-[14px] w-[14px] rounded-full bg-white transition-all",
                          enabled[row.key] ? "left-[16px]" : "left-[2px]",
                        )}
                      />
                    </button>
                    <div className="flex-1">
                      <span className="text-[12.5px] font-semibold">{row.name}</span>
                      <span className="mono ml-2 text-[10.5px] text-ink3">{row.sub}</span>
                      <div className="mt-0.5 text-[11px] leading-snug text-ink3">{row.reason}</div>
                    </div>
                    <span className="mono text-[12px] text-ink1">
                      {row.key === "assets" ? "~" : ""}
                      {fmtMoney(rowCents(row))}/mo
                    </span>
                  </div>
                ))}
                <button
                  type="button"
                  className="mt-2 flex items-center gap-2 text-[12px] font-medium text-steel"
                >
                  <Icon id="s-plus" className="h-3.5 w-3.5" />
                  Add another service
                </button>
              </Card>

              <div className="flex items-center gap-3">
                <Btn
                  variant="p"
                  onClick={submit}
                  disabled={createProject.isPending || !name.trim()}
                >
                  Create project — {fmtMoney(totalCents)}/mo est.
                </Btn>
                <Link to="/$org/settings/templates/new" params={{ org }}>
                  <Btn variant="s">Save as template instead (T3)</Btn>
                </Link>
                <span className="flex items-center gap-2 text-[11px] text-ink3">
                  or:{" "}
                  <Copybit>{`steloit project create ${name || "ecommerce"} --template store`}</Copybit>
                </span>
              </div>
            </div>

            <div className="flex w-[320px] shrink-0 flex-col gap-3">
              <Card className="flex flex-col gap-2 p-4">
                <Eyebrow>Estimated monthly cost</Eyebrow>
                <div className="mono text-[26px] font-semibold tracking-[-0.5px]">
                  {fmtMoney(totalCents)}
                  <span className="ml-1 text-[11px] font-normal text-ink3">
                    /month · production only
                  </span>
                </div>
                <div className="mt-1 flex flex-col gap-1.5">
                  {[
                    { label: "db-main · PostgreSQL", key: "db-main" },
                    { label: "api + worker · compute", key: "compute" },
                    { label: "cache · Valkey", key: "cache" },
                    { label: "jobs · Queue", key: "jobs" },
                    { label: "assets · Storage (usage)", key: "assets" },
                  ].map((line) => {
                    const row = RECOMMENDED.find((r) => r.key === line.key);
                    if (!row || !enabled[line.key]) return null;
                    return (
                      <div key={line.key} className="flex justify-between text-[11.5px]">
                        <span className="text-ink2">{line.label}</span>
                        <span className="mono">
                          {line.key === "assets" ? "~" : ""}
                          {fmtMoney(rowCents(row))}
                        </span>
                      </div>
                    );
                  })}
                </div>
                <p className="mt-1 border-hair border-t pt-2.5 text-[10.5px] leading-relaxed text-ink3">
                  Estimates update as you edit. Nothing exists — and nothing bills — until you
                  confirm. Backups, metrics, logs and alerts are included, not add-ons.
                </p>
              </Card>
              <Card className="border-assist/40 p-4 text-[11.5px] leading-relaxed text-ink2">
                Bindings will be pre-wired: <b>api → db-main, cache, assets</b> ·{" "}
                <b>worker → jobs, db-main</b>. Each gets least-privilege credentials, rotated
                automatically.
              </Card>
            </div>
          </div>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/new-project")({
  component: NewProjectPage,
});
