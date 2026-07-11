import { useMutation, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import type { IconId } from "@/design-system/icon";
import { Pill } from "@/design-system/pill";
import {
  type BlockState,
  type CreatableProduct,
  TYPE_BLOCKS,
  TypeBlock,
} from "@/features/create/type-blocks";
import { useProjects } from "@/features/projects/hooks";
import {
  createEstimateMutation,
  createServiceMutation,
  errorMessage,
  errorRemediation,
  listServicesQueryKey,
} from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * C1–C4 · The ONE create surface (ADR-014): /$org/create. Unselected is the
 * product picker (C1); ?type= renders the per-product type block as a state
 * of the same skeleton (C2/C3/C9–C12). Estimate-before-provision: the confirm
 * button stays disabled until an estimate id exists; success lands on the
 * service page, which renders the C4 provisioning state.
 */

const CREATABLE = ["postgres", "valkey", "storage", "queue", "web", "worker"] as const;

const isCreatable = (v: unknown): v is CreatableProduct =>
  typeof v === "string" && (CREATABLE as readonly string[]).includes(v);

/** C1 product grid — copy and from-prices verbatim. */
const PRODUCT_GRID: {
  type: CreatableProduct;
  icon: IconId;
  name: string;
  desc: string;
  price: string;
}[] = [
  {
    type: "postgres",
    icon: "s-db",
    name: "PostgreSQL",
    desc: "Branchable Postgres — PITR, query insights, read replicas.",
    price: "from $24/mo · Dev",
  },
  {
    type: "valkey",
    icon: "s-chip",
    name: "Valkey",
    desc: "Cache or persistent modes, eviction stated up front.",
    price: "from $11/mo · 512 MB",
  },
  {
    type: "storage",
    icon: "s-bucket",
    name: "Storage",
    desc: "Objects with lifecycle rules; public needs approval.",
    price: "$0.023 / GB-mo",
  },
  {
    type: "queue",
    icon: "s-queue",
    name: "Queue",
    desc: "At-least-once with a DLQ included, contract first.",
    price: "from $4/mo",
  },
  {
    type: "web",
    icon: "s-globe",
    name: "Web",
    desc: "Deploy from Git — Dockerfile detected, TLS automatic.",
    price: "from $17/mo",
  },
  {
    type: "worker",
    icon: "s-worker",
    name: "Worker",
    desc: "Background jobs & crons; scale-to-zero.",
    price: "from $3/mo",
  },
];

const C1_STEPS = [
  "1 · Select product",
  "2 · Configure — the type block adapts",
  "3 · Estimate updates live",
  "4 · Confirm — the only click that creates",
];

const GPU_REASON = "Region exception flow (C7) lands in Phase 3";

function CreatePage() {
  const { org } = Route.useParams();
  const { type, env } = Route.useSearch();
  const navigate = useNavigate();
  const projects = useProjects(org);
  const project = projects.data?.[0]?.name;

  const queryClient = useQueryClient();
  const estimate = useMutation(createEstimateMutation());
  const create = useMutation(createServiceMutation());
  // The estimate is keyed to the product it priced — a product swap invalidates
  // it without a reset effect (which would clobber the child block's mount
  // push: child effects run before parent effects).
  const [est, setEst] = useState<{ forType: string; id: string }>();
  const [block, setBlock] = useState<BlockState>();
  const onBlockChange = useCallback((s: BlockState) => setBlock(s), []);
  const estimateId = type && est?.forType === type ? est.id : undefined;

  // Estimate-before-provision: the type block's first props-up state prices itself.
  useEffect(() => {
    if (!type || !block || estimateId || estimate.isPending) return;
    estimate.mutate(
      { body: { env, services: [{ product: type, name: block.name, shape: block.shape }] } },
      { onSuccess: (data) => setEst({ forType: type, id: data.id }) },
    );
  }, [type, block, estimateId, estimate, env]);

  const pick = (t: CreatableProduct) =>
    navigate({ to: "/$org/create", params: { org }, search: { type: t, env } });

  const cancel = () => {
    if (project) {
      navigate({ to: "/$org/$project", params: { org, project }, search: { env } });
    } else {
      navigate({ to: "/$org", params: { org } });
    }
  };

  const submit = () => {
    if (!type || !block || !estimateId || !project) return;
    const name = block.name;
    create.mutate(
      {
        path: { env },
        body: {
          product: type,
          name,
          shape: block.shape,
          estimate_id: estimateId,
          bindings: block.bindings,
        },
      },
      {
        onSuccess: () => {
          queryClient.invalidateQueries({ queryKey: listServicesQueryKey({ path: { env } }) });
          toast.success(`Provisioning ${name} — billing starts at ready, never at click`);
          navigate({
            to: "/$org/$project/svc/$service",
            params: { org, project, service: name },
            search: { env },
          });
        },
        onError: (err) => {
          const remediation = errorRemediation(err);
          toast.error(errorMessage(err), remediation ? { description: remediation } : undefined);
        },
      },
    );
  };

  const def = type ? TYPE_BLOCKS[type] : undefined;

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        {type && def ? (
          /* ---------- selected · C2/C3/C9–C12 ---------- */
          <>
            <div className="mb-3 flex items-center gap-2">
              <div className="chiprow">
                {(Object.keys(TYPE_BLOCKS) as CreatableProduct[]).map((p) => (
                  <button
                    key={p}
                    type="button"
                    aria-pressed={p === type}
                    className={cn("chip", p === type && "on")}
                    onClick={() => pick(p)}
                  >
                    {TYPE_BLOCKS[p].label}
                  </button>
                ))}
                <button type="button" className="chip opacity-60" disabled title={GPU_REASON}>
                  GPU Worker <Pill tone="st">beta</Pill>
                </button>
              </div>
              <span className="sp" />
              <span className="mono text-[10.5px] text-ink3">
                ⌘K "add {type}" lands here preselected
              </span>
            </div>
            <Pghead
              title={def.h1}
              sub={`in ${org} / ${env} · home region aws · ap-south-1${def.hsubSuffix}`}
            />
            <div className="flex gap-4">
              <div className="flex max-w-[760px] flex-1 flex-col gap-3.5">
                <TypeBlock key={type} product={type} onChange={onBlockChange} />
              </div>
              <div className="flex w-[310px] shrink-0 flex-col gap-3">
                <Card className="flex flex-col gap-2 p-4">
                  <Eyebrow>Estimated monthly cost</Eyebrow>
                  <div className="mono text-[26px] font-semibold tracking-[-0.5px]">
                    {block?.totalLabel ?? "—"}
                    <span className="ml-1 text-[11px] font-normal text-ink3">/mo</span>
                  </div>
                  <div className="mt-1 flex flex-col gap-1.5">
                    {(block?.lines ?? []).map((line) => (
                      <div key={line.label} className="flex justify-between text-[11.5px]">
                        <span className="text-ink2">{line.label}</span>
                        <span className="mono">{line.amount}</span>
                      </div>
                    ))}
                  </div>
                  {def.footer ? (
                    <p className="mt-1 border-hair border-t pt-2.5 text-[10.5px] leading-relaxed text-ink3">
                      {def.footer}
                    </p>
                  ) : null}
                  <Btn
                    variant="p"
                    className="mt-1 justify-center"
                    onClick={submit}
                    disabled={!estimateId || !block || create.isPending}
                    disabledReason="Waiting for the estimate — nothing provisions without one"
                  >
                    {block?.buttonLabel ?? "Create"}
                  </Btn>
                  <Btn variant="s" className="justify-center" onClick={cancel}>
                    Cancel
                  </Btn>
                  {block ? (
                    <div className="flex justify-center">
                      <Copybit>{block.cli}</Copybit>
                    </div>
                  ) : null}
                </Card>
                {type === "valkey" ? (
                  <Card className="p-4 text-[11.5px] leading-relaxed text-ink2">
                    The same skeleton serves every product: name → type block → bindings → included
                    defaults → estimate → confirm. Storage adds bucket policy; Queue adds delivery &
                    DLQ; Compute adds repo & health checks.
                  </Card>
                ) : null}
              </div>
            </div>
          </>
        ) : (
          /* ---------- unselected · C1 ---------- */
          <>
            <Pghead
              title="New instance"
              sub={`in ${org} / ${env} · pick a product — the form below adapts, the skeleton never changes: name → type → bindings → defaults → estimate → confirm`}
            />
            <div className="flex gap-4">
              <div className="flex max-w-[820px] flex-1 flex-col gap-3.5">
                <Card
                  className="flex items-center gap-3 border-assist/40 p-4 opacity-75"
                  title="Describe-to-provision (AI1) lands in Phase 3"
                >
                  <Glyph id="s-ai" />
                  <span className="flex-1 text-[12px] leading-relaxed text-ink2">
                    Not sure what you need? Describe what you're building and the assistant suggests
                    a set — you approve every line.
                  </span>
                  <Pill tone="ai">optional</Pill>
                </Card>
                <div className="ordiv">or pick a product</div>
                <div className="grid grid-cols-3 gap-2.5">
                  {PRODUCT_GRID.map((p) => (
                    <button
                      key={p.type}
                      type="button"
                      onClick={() => pick(p.type)}
                      className="card flex flex-col gap-1.5 p-3 text-left hover:border-ink3"
                    >
                      <span className="flex items-center gap-2 text-[12.5px] font-semibold">
                        <Glyph id={p.icon} />
                        {p.name}
                      </span>
                      <span className="text-[10.5px] leading-snug text-ink3">{p.desc}</span>
                      <span className="mono text-[10.5px] text-ink2">{p.price}</span>
                    </button>
                  ))}
                </div>
                <div className="grid grid-cols-3 gap-2.5">
                  <button
                    type="button"
                    disabled
                    title={GPU_REASON}
                    className="card flex flex-col gap-1.5 p-3 text-left opacity-75"
                  >
                    <span className="flex items-center gap-1.5 text-[12.5px] font-semibold">
                      GPU Worker <Pill tone="st">beta</Pill>
                    </span>
                    <span className="self-start">
                      <Pill tone="warn">not in ap-south-1 yet — request it (C7)</Pill>
                    </span>
                    <span className="text-[10.5px] leading-snug text-ink3">Batch GPU jobs.</span>
                    <span className="mono text-[10.5px] text-ink2">
                      from $88/mo · nearest cell aws · ap-southeast-1 (+34 ms)
                    </span>
                  </button>
                  <Card dashed className="col-span-2 flex flex-col gap-1.5 p-3">
                    <span className="text-[12.5px] font-semibold">Start from a template</span>
                    <span className="text-[10.5px] leading-snug text-ink3">
                      saas-starter $96 · docs-site $14 · store $184 — whole stacks, each opening a
                      full estimate first.
                    </span>
                    <span className="self-start">
                      <Btn
                        variant="gh"
                        disabled
                        disabledReason="Templates gallery (C5) lands in Phase 3"
                        className="!h-auto !p-0 text-steel"
                      >
                        Browse templates →
                      </Btn>
                    </span>
                  </Card>
                </div>
              </div>
              <div className="flex w-[320px] shrink-0 flex-col gap-3">
                <Card className="flex flex-col gap-2 p-4">
                  <Eyebrow>Estimate</Eyebrow>
                  <div className="mono text-[26px] font-semibold tracking-[-0.5px]">
                    $0
                    <span className="ml-1 text-[11px] font-normal text-ink3">/mo</span>
                  </div>
                  <p className="text-[11.5px] leading-relaxed text-ink2">
                    Nothing exists yet. Pick a product and the estimate builds as you configure —
                    billing starts only at ready, never at click.
                  </p>
                  <div className="mt-1 flex flex-col gap-1.5 border-hair border-t pt-2.5">
                    {C1_STEPS.map((step) => (
                      <div key={step} className="text-[11.5px] text-ink2">
                        {step}
                      </div>
                    ))}
                  </div>
                  <Btn variant="s" className="mt-1 justify-center" onClick={cancel}>
                    Cancel
                  </Btn>
                  <div className="flex justify-center">
                    <Copybit>steloit add postgres --dry-run</Copybit>
                  </div>
                </Card>
              </div>
            </div>
          </>
        )}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/create")({
  validateSearch: (search: Record<string, unknown>): { type?: CreatableProduct; env: string } => ({
    type: isCreatable(search.type) ? search.type : undefined,
    env: typeof search.env === "string" ? search.env : "production",
  }),
  component: CreatePage,
});
