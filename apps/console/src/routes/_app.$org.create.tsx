import { useMutation, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Banner } from "@/design-system/banner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Icon, type IconId } from "@/design-system/icon";
import { Inp } from "@/design-system/inp";
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

const CREATABLE = [
  "postgres",
  "valkey",
  "storage",
  "queue",
  "web",
  "worker",
  "gpu-worker",
] as const;

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

/* ---------- AI1 · describe-to-provision (Law 1: suggest, never act) ---------- */

const AI1_DEFAULT_TEXT =
  "I'm building a SaaS app with authentication, file uploads and background jobs.";

type Ai1Enabled = Record<string, boolean>;

const AI1_SUGGESTIONS: {
  key: string;
  icon: IconId;
  name: string;
  tag: string;
  why: string;
  price: number;
  defaultOn: boolean;
}[] = [
  {
    key: "postgres",
    icon: "s-db",
    name: "PostgreSQL",
    tag: "Standard · db",
    why: "durable store for accounts, sessions and app data — the SaaS backbone",
    price: 58,
    defaultOn: true,
  },
  {
    key: "storage",
    icon: "s-bucket",
    name: "Object Storage",
    tag: "with lifecycle",
    why: "file uploads, with a lifecycle rule so temp files expire — you named uploads",
    price: 9,
    defaultOn: true,
  },
  {
    key: "queue",
    icon: "s-queue",
    name: "Queue",
    tag: "DLQ included",
    why: "background jobs run off the request path; DLQ catches failures",
    price: 12,
    defaultOn: true,
  },
  {
    key: "valkey",
    icon: "s-chip",
    name: "Valkey",
    tag: "cache · sessions",
    why: "session store + cache — optional now, cheap to add later",
    price: 22,
    defaultOn: false,
  },
];

const AI1_DEFAULTS: Ai1Enabled = Object.fromEntries(
  AI1_SUGGESTIONS.map((s) => [s.key, s.defaultOn]),
);

/** The working describe card + suggestion block; the estimate rail flip lives in CreatePage. */
function DescribeToProvision({
  enabled,
  onSuggest,
  onToggle,
}: {
  enabled: Ai1Enabled | null;
  onSuggest: () => void;
  onToggle: (key: string) => void;
}) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const [text, setText] = useState(AI1_DEFAULT_TEXT);

  return (
    <>
      {/* biome-ignore lint/a11y/noStaticElementInteractions: the click is a focus affordance; the input inside is the interactive element */}
      <div
        ref={wrapRef}
        onClick={() => wrapRef.current?.querySelector("input")?.focus()}
        role="presentation"
      >
        <Card className="flex flex-col gap-2.5 border-assist/40 p-4">
          <div className="flex items-center gap-3">
            <Glyph id="s-ai" />
            <span className="flex-1 text-12 leading-relaxed text-ink2">
              Not sure what you need? Describe what you're building and the assistant suggests a set
              — you approve every line.
            </span>
            <Pill tone="ai">optional</Pill>
          </div>
          <Inp
            value={text}
            onChange={(e) => setText(e.target.value)}
            className="text-12"
            aria-label="Describe what you're building"
          />
          <div className="flex items-center gap-2">
            <Btn variant="a" onClick={onSuggest}>
              Suggest a setup
            </Btn>
            <span className="mono text-10 text-ink3">
              nothing is created — you get a reviewable set
            </span>
          </div>
        </Card>
      </div>

      {enabled ? (
        <>
          <div className="flex items-center gap-2">
            <Eyebrow className="m-0">Suggested for your description</Eyebrow>
            <Pill tone="ai">4 services · each explainable</Pill>
            <span className="sp flex-1" />
            <span className="mono text-10 text-ink3">toggle any off — estimate updates live</span>
          </div>
          <div className="flex flex-col gap-2">
            {AI1_SUGGESTIONS.map((s) => {
              const on = enabled[s.key];
              return (
                <button
                  key={s.key}
                  type="button"
                  aria-pressed={on}
                  onClick={() => onToggle(s.key)}
                  className={cn("card flex items-center gap-3 p-3 text-left", !on && "opacity-55")}
                >
                  <span
                    className={cn(
                      "flex h-[15px] w-[15px] shrink-0 items-center justify-center rounded",
                      on ? "bg-steel" : "border-[1.5px] border-hair",
                    )}
                  >
                    {on ? <Icon id="s-check" className="h-2.5 w-2.5 text-white" /> : null}
                  </span>
                  <Glyph id={s.icon} />
                  <span className="flex-1">
                    <span className="flex items-center gap-2">
                      <b className="text-12">{s.name}</b>
                      <span className="mono text-10 text-ink3">{s.tag}</span>
                    </span>
                    <span className="mt-0.5 block text-10p5 text-ink2">
                      <span className="text-assist">Why:</span> {s.why}
                    </span>
                  </span>
                  <span className="mono text-11 text-ink2">${s.price}</span>
                </button>
              );
            })}
          </div>
        </>
      ) : null}
    </>
  );
}

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
  // AI1: null = no suggestion yet; a toggle map = the reviewable set (Law 1).
  const [ai1, setAi1] = useState<Ai1Enabled | null>(null);
  const onBlockChange = useCallback((s: BlockState) => setBlock(s), []);
  const estimateId = type && est?.forType === type ? est.id : undefined;

  // Estimate-before-provision: the type block's first props-up state prices
  // itself. isError stops the loop — without it a failing estimate re-fires on
  // every render forever; recovery is the explicit retry in the rail.
  useEffect(() => {
    if (!type || !block || estimateId || estimate.isPending || estimate.isError) return;
    estimate.mutate(
      { body: { env, services: [{ product: type, name: block.name, shape: block.shape }] } },
      { onSuccess: (data) => setEst({ forType: type, id: data.id }) },
    );
  }, [type, block, estimateId, estimate, env]);

  const retryEstimate = () => {
    if (!type || !block) return;
    estimate.mutate(
      { body: { env, services: [{ product: type, name: block.name, shape: block.shape }] } },
      { onSuccess: (data) => setEst({ forType: type, id: data.id }) },
    );
  };

  const pick = (t: CreatableProduct) => {
    // A stale failure must not gate the next product's auto-estimate.
    if (estimate.isError) estimate.reset();
    navigate({ to: "/$org/create", params: { org }, search: { type: t, env } });
  };

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
                {(Object.keys(TYPE_BLOCKS) as CreatableProduct[])
                  .filter((p) => p !== "gpu-worker")
                  .map((p) => (
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
                <button
                  type="button"
                  aria-pressed={type === "gpu-worker"}
                  className={cn("chip", type === "gpu-worker" && "on")}
                  onClick={() => pick("gpu-worker")}
                >
                  GPU Worker <Pill tone="st">beta</Pill>
                </button>
              </div>
              <span className="sp" />
              <span className="mono text-10p5 text-ink3">
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
                    <span className="ml-1 text-11 font-normal text-ink3">/mo</span>
                  </div>
                  <div className="mt-1 flex flex-col gap-1.5">
                    {(block?.lines ?? []).map((line) => (
                      <div key={line.label} className="flex justify-between text-11p5">
                        <span className="text-ink2">{line.label}</span>
                        <span className="mono">{line.amount}</span>
                      </div>
                    ))}
                  </div>
                  {def.footer ? (
                    <p className="mt-1 border-hair border-t pt-2.5 text-10p5 leading-relaxed text-ink3">
                      {def.footer}
                    </p>
                  ) : null}
                  {/* Estimate failure is loud, not "Waiting…" forever (audit
                      P1): the banner names it, retry re-fires, the confirm
                      button's reason points here. */}
                  {estimate.isError ? (
                    <Banner tone="warn">
                      <span className="flex-1">
                        Estimate failed — {errorMessage(estimate.error)}
                      </span>
                      <Btn variant="s" className="h-6 px-2 text-10p5" onClick={retryEstimate}>
                        Retry
                      </Btn>
                    </Banner>
                  ) : null}
                  <Btn
                    variant="p"
                    className="mt-1 justify-center"
                    onClick={submit}
                    disabled={!estimateId || !block || create.isPending}
                    disabledReason={
                      estimate.isError
                        ? "Estimate failed — retry above"
                        : "Waiting for the estimate — nothing provisions without one"
                    }
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
                  <Card className="p-4 text-11p5 leading-relaxed text-ink2">
                    The same skeleton serves every product: name → type block → bindings → included
                    defaults → estimate → confirm. Storage adds bucket policy; Queue adds delivery &
                    DLQ; Compute adds repo & health checks.
                  </Card>
                ) : null}
                {type === "gpu-worker" ? (
                  <Card className="p-4 text-10p5 leading-relaxed text-ink3">
                    Region model:{" "}
                    <b className="text-ink2">
                      env sets the home · instances inherit · exceptions are explicit
                    </b>{" "}
                    — offered only for availability gaps and typed cross-region features (read
                    replicas, multi-region buckets).
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
                <DescribeToProvision
                  enabled={ai1}
                  onSuggest={() => setAi1(AI1_DEFAULTS)}
                  onToggle={(key) => setAi1((prev) => prev && { ...prev, [key]: !prev[key] })}
                />
                <div className="ordiv">or pick a product</div>
                <div className="grid grid-cols-3 gap-2.5">
                  {PRODUCT_GRID.map((p) => (
                    <button
                      key={p.type}
                      type="button"
                      onClick={() => pick(p.type)}
                      className="card flex flex-col gap-1.5 p-3 text-left hover:border-ink3"
                    >
                      <span className="flex items-center gap-2 text-12p5 font-semibold">
                        <Glyph id={p.icon} />
                        {p.name}
                      </span>
                      <span className="text-10p5 leading-snug text-ink3">{p.desc}</span>
                      <span className="mono text-10p5 text-ink2">{p.price}</span>
                    </button>
                  ))}
                </div>
                <div className="grid grid-cols-3 gap-2.5">
                  <button
                    type="button"
                    onClick={() => pick("gpu-worker")}
                    className="card flex flex-col gap-1.5 p-3 text-left hover:border-ink3"
                  >
                    <span className="flex items-center gap-1.5 text-12p5 font-semibold">
                      GPU Worker <Pill tone="st">beta</Pill>
                    </span>
                    <span className="self-start">
                      <Pill tone="warn">not in ap-south-1 yet — request it (C7)</Pill>
                    </span>
                    <span className="text-10p5 leading-snug text-ink3">Batch GPU jobs.</span>
                    <span className="mono text-10p5 text-ink2">
                      from $88/mo · nearest cell aws · ap-southeast-1 (+34 ms)
                    </span>
                  </button>
                  <Card dashed className="col-span-2 flex flex-col gap-1.5 p-3">
                    <span className="text-12p5 font-semibold">Start from a template</span>
                    <span className="text-10p5 leading-snug text-ink3">
                      saas-starter $96 · docs-site $14 · store $184 — whole stacks, each opening a
                      full estimate first.
                    </span>
                    <Link to="/$org/new-project/templates" params={{ org }} className="self-start">
                      <Btn variant="gh" className="!h-auto !p-0 text-steel">
                        Browse templates →
                      </Btn>
                    </Link>
                  </Card>
                </div>
              </div>
              <div className="flex w-[320px] shrink-0 flex-col gap-3">
                {ai1 ? (
                  (() => {
                    const on = AI1_SUGGESTIONS.filter((s) => ai1[s.key]);
                    const off = AI1_SUGGESTIONS.filter((s) => !ai1[s.key]);
                    const total = on.reduce((sum, s) => sum + s.price, 0);
                    return (
                      <Card className="flex flex-col gap-2 p-4">
                        <Eyebrow className="text-steel">
                          Estimate · {on.length} of 4 enabled
                        </Eyebrow>
                        <div className="mono text-[26px] font-semibold tracking-[-0.5px]">
                          ${total}
                          <span className="ml-1 text-11 font-normal text-ink3">/mo</span>
                        </div>
                        <div className="mt-1 flex flex-col gap-1.5">
                          {on.map((s) => (
                            <div key={s.key} className="flex justify-between text-11p5">
                              <span className="text-ink2">{s.name}</span>
                              <span className="mono">${s.price}</span>
                            </div>
                          ))}
                          {off.map((s) => (
                            <div key={s.key} className="flex justify-between text-11p5 opacity-50">
                              <span className="text-ink2">{s.name} · off</span>
                              <span className="mono">+${s.price}</span>
                            </div>
                          ))}
                        </div>
                        <p className="mt-1 border-hair border-t pt-2.5 text-10 leading-relaxed text-ink3">
                          The assistant proposed; you decide. Nothing provisions until you confirm —{" "}
                          <b>billing starts at ready, not now</b>.
                        </p>
                        <Btn
                          variant="p"
                          className="justify-center"
                          disabled
                          disabledReason="Multi-service create lands with AI1's batch endpoint (finding) — create each via its type block today"
                        >
                          Review & create {on.length} services →
                        </Btn>
                        <Btn variant="s" className="justify-center" onClick={() => setAi1(null)}>
                          Clear suggestion
                        </Btn>
                        <p className="text-center text-10 leading-relaxed text-ink3">
                          Law 1: AI suggests, you decide. Law 2: every "Why" is shown. This whole
                          panel is off if your org disabled AI (P-policy).
                        </p>
                      </Card>
                    );
                  })()
                ) : (
                  <Card className="flex flex-col gap-2 p-4">
                    <Eyebrow>Estimate</Eyebrow>
                    <div className="mono text-[26px] font-semibold tracking-[-0.5px]">
                      $0
                      <span className="ml-1 text-11 font-normal text-ink3">/mo</span>
                    </div>
                    <p className="text-11p5 leading-relaxed text-ink2">
                      Nothing exists yet. Pick a product and the estimate builds as you configure —
                      billing starts only at ready, never at click.
                    </p>
                    <div className="mt-1 flex flex-col gap-1.5 border-hair border-t pt-2.5">
                      {C1_STEPS.map((step) => (
                        <div key={step} className="text-11p5 text-ink2">
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
                )}
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
