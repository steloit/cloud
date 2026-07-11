import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Icon, type IconId } from "@/design-system/icon";
import { Flabel, Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { errorMessage, getTemplateOptions, refreshTemplateMutation } from "@/lib/api";
import { fmtMoney } from "@/lib/fmt";

/**
 * C6 · Template detail — review everything, rename, then it feeds the guided
 * flow (W2). "store" is the canon exemplar and is frame-fixed (it is a gallery
 * template, not in fixtures); other :tpl params render from getTemplateOptions
 * with an honest reduced layout. No snav — plain two-column main.
 */

/** The store services table — frame-fixed (C6), copy verbatim. */
const STORE_SERVICES: {
  icon: IconId;
  name: string;
  type: string;
  why: string;
  cost: string;
}[] = [
  {
    icon: "s-db",
    name: "db-main",
    type: "PostgreSQL · 2 vCPU / 4 GB",
    why: "Relational core · migrations included",
    cost: "$58",
  },
  {
    icon: "s-chip",
    name: "cache",
    type: "Valkey · cache · 1 GB",
    why: "Sessions & carts",
    cost: "$22",
  },
  {
    icon: "s-bucket",
    name: "assets",
    type: "Object Storage",
    why: "Product images · signed URLs",
    cost: "~$9",
  },
  { icon: "s-queue", name: "jobs", type: "Queue · DLQ on", why: "Orders & receipts", cost: "$12" },
  {
    icon: "s-globe",
    name: "api + worker",
    type: "Web service + Worker",
    why: "From the example repo",
    cost: "$83",
  },
];

const STORE_BINDINGS = [
  "api → db-main · rw",
  "api → cache · rw",
  "api → assets · write",
  "worker → jobs · consume",
  "worker → db-main · rw",
];

function StoreDetail({ org }: { org: string }) {
  const [name, setName] = useState("my-store");
  return (
    <div className="flex gap-4">
      <div className="flex max-w-[740px] flex-1 flex-col gap-3.5">
        <div className="flex items-center gap-3">
          <span className="glyph h-[42px] w-[42px]" style={{ background: "var(--steel-tint)" }}>
            <Icon id="s-hex" className="!h-5 !w-5 text-steel" />
          </span>
          <div>
            <h1 className="h1 flex items-center gap-2.5">
              store <Pill tone="st">featured</Pill>
              <Pill tone="mut">by Steloit</Pill>
            </h1>
            <div className="hsub">Full commerce stack · 1.2k deploys · updated 2 weeks ago</div>
          </div>
        </div>
        <p className="text-[12.5px] leading-relaxed text-ink2">
          Catalog, users and orders on PostgreSQL; session carts on Valkey; product images in Object
          Storage; order processing and receipts through a Queue consumed by a worker. Example
          TypeScript app included, deployable on first push.
        </p>
        <div className="tblwrap">
          <table className="tbl">
            <thead>
              <tr>
                <th>Service</th>
                <th>Type & size</th>
                <th>Why it's here</th>
                <th>Cost / mo</th>
              </tr>
            </thead>
            <tbody>
              {STORE_SERVICES.map((s) => (
                <tr key={s.name}>
                  <td>
                    <span className="inline-flex items-center gap-2">
                      <Icon id={s.icon} className="h-[13px] w-[13px] text-ink3" />
                      <b>{s.name}</b>
                    </span>
                  </td>
                  <td className="mono text-[11px] text-ink2">{s.type}</td>
                  <td className="text-[11px] text-ink2">{s.why}</td>
                  <td className="mono text-[11px]">{s.cost}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Eyebrow className="mr-1">Pre-wired bindings</Eyebrow>
          {STORE_BINDINGS.map((b) => (
            <span key={b} className="bindpill">
              <Icon id="s-bind" />
              {b}
            </span>
          ))}
        </div>
      </div>
      <div className="flex w-[320px] shrink-0 flex-col gap-3">
        <Card className="flex flex-col gap-2 p-4">
          <Eyebrow>Estimated monthly cost</Eyebrow>
          <div className="mono text-[28px] font-semibold tracking-[-0.5px]">
            $184
            <span className="ml-1.5 text-[11px] font-normal text-ink3">
              /month · production only
            </span>
          </div>
          <p className="mt-1 border-hair border-t pt-2.5 text-[10.5px] leading-relaxed text-ink3">
            You'll review and edit every service in the guided flow before confirming — the template
            only pre-fills it.
          </p>
        </Card>
        <div>
          <Flabel htmlFor="tpl-project-name">Project name</Flabel>
          <Inp
            id="tpl-project-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="foc"
          />
        </div>
        {/* Template-prefilled create lands in Phase 5 — for now the button opens the guided flow (W2). */}
        <Link to="/$org/new-project" params={{ org }}>
          <Btn variant="p" className="h-[38px] w-full justify-center">
            Use this template →
          </Btn>
        </Link>
        <div className="flex justify-center">
          <Copybit>steloit init --template store</Copybit>
        </div>
        <Card className="p-3 text-[10.5px] leading-relaxed text-ink3">
          Example code: <span className="mono text-ink2">github.com/steloit/templates/store</span> ·
          TypeScript · MIT. Forked into your repo on create.
        </Card>
      </div>
    </div>
  );
}

/**
 * T2 · Org template detail — a frozen copy of the shape, not a live link to
 * the source. Contents rows for store-baseline are frame-fixed (T2); other
 * org templates render from the API. Frame id tpl_9c22e1 vs fixtures'
 * tpl_store_baseline — data wins (finding).
 */
const T2_CONTENTS = [
  {
    name: "db-main",
    type: "PostgreSQL 16",
    shape: "Standard · 2 vCPU / 4 GB · pgvector, pg_cron",
    est: "$58",
  },
  { name: "db-reports", type: "PostgreSQL 16", shape: "Dev · 1 vCPU / 2 GB", est: "$24" },
  { name: "cache", type: "Valkey", shape: "2 GB · allkeys-lru", est: "$22" },
  { name: "assets", type: "Object Storage", shape: "lifecycle: tmp 7d expire", est: "$9" },
  { name: "jobs", type: "Queue", shape: "DLQ on · 3 schedules", est: "$12" },
  {
    name: "api + worker",
    type: "Web + Worker",
    shape: "autoscale 2–6 · bindings pre-wired ⇒ placeholders",
    est: "$83",
  },
];

function OrgTemplateDetail({ org, tpl }: { org: string; tpl: string }) {
  const template = useQuery(getTemplateOptions({ path: { tpl } }));
  const refresh = useMutation(refreshTemplateMutation());
  const queryClient = useQueryClient();

  if (template.isLoading) return <div className="text-[11.5px] text-ink3">Loading template…</div>;
  if (!template.data) {
    return (
      <Card dashed className="max-w-[560px] p-6 text-[12px] text-ink2">
        Template <span className="mono">{tpl}</span> wasn't found. Gallery detail exists only for{" "}
        <b>store</b> — org templates are listed under Settings → Templates.
      </Card>
    );
  }
  const t = template.data;
  const isStoreBaseline = t.name === "store-baseline";

  return (
    <div className="flex gap-4">
      <div className="flex max-w-[740px] flex-1 flex-col gap-3.5">
        <div className="flex items-center gap-3">
          <div className="flex-1">
            <h1 className="h1 flex items-center gap-2.5">
              Templates · {t.name}{" "}
              {t.visibility === "org" ? (
                <Pill tone="st">org</Pill>
              ) : (
                <Pill tone="mut">restricted</Pill>
              )}
            </h1>
            <div className="hsub">
              Saved from{" "}
              <span className="mono">
                {t.source?.project?.replace("prj_", "") ?? "…"} / production
              </span>{" "}
              · v{t.version} · updated Mar 12 by asha · used {t.used_by_count ?? 0}× — a frozen copy
              of the shape, not a live link to the source.
            </div>
          </div>
          <Btn
            variant="s"
            disabled={refresh.isPending}
            onClick={() =>
              refresh.mutate(
                { path: { tpl: t.id } },
                {
                  onSuccess: (data) => {
                    toast.success(`Refreshed from source — now v${data.version}`);
                    queryClient.invalidateQueries({
                      queryKey: getTemplateOptions({ path: { tpl } }).queryKey,
                    });
                  },
                  onError: (err) => toast.error(errorMessage(err)),
                },
              )
            }
          >
            Refresh from source…
          </Btn>
          <Btn variant="s" disabled disabledReason="Duplicate lands with template editing">
            Duplicate
          </Btn>
          <Link to="/$org/new-project" params={{ org }}>
            <Btn variant="p">Use this template →</Btn>
          </Link>
        </div>
        <Card>
          <div className="border-hair border-b px-4 py-2.5">
            <Eyebrow>Contents — editable per service</Eyebrow>
          </div>
          <table className="tbl">
            <thead>
              <tr>
                <th>Service</th>
                <th>Shape</th>
                <th>Est. / mo</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {(isStoreBaseline ? T2_CONTENTS : []).map((row) => (
                <tr key={row.name}>
                  <td>
                    <b>{row.name}</b> <span className="text-ink3">· {row.type}</span>
                  </td>
                  <td className="text-ink2">{row.shape}</td>
                  <td className="mono">{row.est}</td>
                  <td>
                    <Btn
                      variant="gh"
                      className="h-6 px-2 text-[10.5px]"
                      disabled
                      disabledReason="Shape editing lands with template versioning"
                    >
                      Edit shape
                    </Btn>
                  </td>
                </tr>
              ))}
              {!isStoreBaseline ? (
                <tr>
                  <td colSpan={4} className="text-ink3">
                    {typeof t.contents?.services === "number"
                      ? `${t.contents.services} services — per-service rows are frame-fixed only for store-baseline`
                      : "saved shape"}
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
          <div className="flex items-baseline gap-2 border-hair border-t px-4 py-2.5">
            <Eyebrow>Instantiation estimate</Eyebrow>
            <span className="mono text-[16px] font-semibold">
              {fmtMoney(t.monthly_estimate_cents ?? 0)}
            </span>
            <span className="text-[10.5px] text-ink3">
              /mo at today's prices — every consumer still sees the full estimate before anything
              provisions
            </span>
          </div>
        </Card>
      </div>
      <div className="flex w-[300px] shrink-0 flex-col gap-3">
        <Card className="flex flex-col gap-1.5 p-3.5">
          <Eyebrow>What's captured</Eyebrow>
          <div className="text-[11.5px] text-ok">✓ Service shapes, versions, extensions</div>
          <div className="text-[11.5px] text-ok">✓ Binding graph — as placeholders</div>
          <div className="text-[11.5px] text-ok">✓ Lifecycle rules, schedules, alerts</div>
          <div className="text-[11.5px] text-err">
            ✗ <b>Never:</b> secrets, data, keys — re-minted per create
          </div>
        </Card>
        <Card className="flex flex-col gap-1.5 p-3.5">
          <Eyebrow>Used by · informational</Eyebrow>
          <p className="text-[11.5px] text-ink2">initech (Mar 2) · borealis/starter (Mar 22).</p>
          <p className="text-[10.5px] text-ink3">
            Copies, not links — editing v{t.version} changes neither.
          </p>
        </Card>
        <Card className="flex flex-col gap-1.5 p-3.5">
          <Eyebrow>Settings</Eyebrow>
          <div className="flex justify-between text-[11.5px]">
            <span className="text-ink3">Visibility</span>
            <Pill tone={t.visibility === "org" ? "st" : "mut"}>{t.visibility}</Pill>
          </div>
          <div className="text-[11.5px] text-ink3">Rename · Delete…</div>
          <div className="mono text-[10.5px] text-ink3">{t.id}</div>
        </Card>
      </div>
    </div>
  );
}

/** C6 (store, frame-fixed) or T2 (org templates, API-backed) by param. */
function TemplateDetailPage() {
  const { org, tpl } = Route.useParams();
  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        {tpl === "store" ? <StoreDetail org={org} /> : <OrgTemplateDetail org={org} tpl={tpl} />}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/template/$tpl")({
  component: TemplateDetailPage,
});
