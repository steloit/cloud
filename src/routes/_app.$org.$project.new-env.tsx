import { useMutation, useQuery } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Icon } from "@/design-system/icon";
import { Flabel, Inp } from "@/design-system/inp";
import { Dot, Pill } from "@/design-system/pill";
import { useEnvironments } from "@/features/projects/hooks";
import { createEnvironmentMutation, errorMessage, listCellsOptions } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * C8 + X3 · New environment — the US-client answer: the whole stack moves near
 * the client, with its own home region. Managed regions and customer cells sit
 * in ONE combined selector (X3) — the only place a developer meets BYOC.
 *
 * Route not in 02-ia — reached from W3's dashed "⊕ New environment" card, M3,
 * and the env crumb; minimal addition, surfaced as a finding.
 *
 * NOTE: MSW has no POST /projects/:project/envs handler yet — the mutation is
 * wired regardless; failures surface as an error toast (finding).
 */

/** Managed regions — frame-fixed (C8). */
const MANAGED = [
  {
    id: "aws/us-east-1",
    provider: "aws",
    title: "aws · us-east-1",
    sub: "N. Virginia · closest to your client",
  },
  { id: "gcp/us-central1", provider: "gcp", title: "gcp · us-central1", sub: "Iowa" },
  {
    id: "aws/ap-south-1",
    provider: "aws",
    title: "aws · ap-south-1",
    sub: "Mumbai · org default · home of production",
  },
];

// Frame-fixed per-cell display (X3): latency strings aren't in the Cell schema.
// Provider/region come from the fixtures (fixtures win for data facts).
const CELL_DISPLAY: Record<string, { latency: string }> = {
  cell_eu: { latency: "12 ms · in-region" },
  cell_prod: { latency: "198 ms" },
};

const FILTERS = ["All", "Managed", "Our clouds", "AWS", "GCP"] as const;
type Filter = (typeof FILTERS)[number];

type Target = { kind: "managed"; id: string } | { kind: "cell"; id: string };

function NewEnvironmentPage() {
  const { org, project } = Route.useParams();
  const navigate = useNavigate();
  const environments = useEnvironments(project);
  const cells = useQuery({ ...listCellsOptions({ path: { org } }), select: (r) => r.data ?? [] });
  const createEnv = useMutation(createEnvironmentMutation());

  const [name, setName] = useState("production-us");
  const [filter, setFilter] = useState<Filter>("All");
  const [search, setSearch] = useState("");
  const [target, setTarget] = useState<Target>({ kind: "managed", id: "aws/us-east-1" });
  const [shape, setShape] = useState<"clone" | "empty">("clone");
  const [data, setData] = useState<"empty" | "branch">("empty");

  const q = search.trim().toLowerCase();
  // X3: cells are healthy deployment targets even with an agent upgrade pending.
  const cellRows = (cells.data ?? [])
    .filter((c) => {
      if (filter === "Managed") return false;
      if (filter === "AWS" && c.provider !== "aws") return false;
      if (filter === "GCP" && c.provider !== "gcp") return false;
      return `${c.name} ${c.region}`.toLowerCase().includes(q);
    })
    .sort(
      (a, b) => (a.id === "cell_eu" ? 0 : 1) - (b.id === "cell_eu" ? 0 : 1), // distance-sorted, frame order
    );
  const managedRows = MANAGED.filter((m) => {
    if (filter === "Our clouds") return false;
    if (filter === "AWS" && m.provider !== "aws") return false;
    if (filter === "GCP" && m.provider !== "gcp") return false;
    return `${m.title} ${m.sub}`.toLowerCase().includes(q);
  });

  const targetCell =
    target.kind === "cell" ? (cells.data ?? []).find((c) => c.id === target.id) : undefined;
  const region = target.kind === "managed" ? target.id : (targetCell?.region ?? "");
  const totalLabel = shape === "clone" ? "$46" : "$0";
  const buttonLabel = targetCell
    ? `Create ${name || "…"} → ${targetCell.name}`
    : `Create ${name || "…"} — ${totalLabel}/mo est.`;
  const cli = `steloit env create ${name || "production-us"} --region ${region}${shape === "clone" ? " --clone-shape production" : ""}`;

  const submit = () => {
    const cloneFrom = environments.data?.find((e) => e.name === "production")?.id;
    createEnv.mutate(
      {
        path: { project },
        body: {
          name,
          region,
          clone_shape_from: shape === "clone" ? cloneFrom : undefined,
          data,
        },
      },
      {
        onSuccess: () => {
          toast.success(`Environment ${name} created — its own network, credentials and data`);
          navigate({ to: "/$org/$project", params: { org, project }, search: { env: name } });
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="New environment"
          sub={
            <>
              in {project} · fully isolated: own private network, own credentials, own data —{" "}
              <b>and its own home region</b>
            </>
          }
        />
        <div className="flex gap-4">
          <div className="flex max-w-[760px] flex-1 flex-col gap-3.5">
            <Card className="flex flex-col p-4">
              <Flabel htmlFor="env-name">Environment name</Flabel>
              <Inp
                id="env-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="foc"
              />
              <div className="mono mt-1.5 text-10p5 text-ink3">
                for the US client launch · promotions can target this environment like any other
              </div>
            </Card>

            <Card className="flex flex-col gap-2.5 p-4">
              <Flabel>
                Home region{" "}
                <span className="font-normal text-ink3">
                  one list, sorted by distance — provider is a facet, filterable but never a gate
                </span>
              </Flabel>
              <div className="flex items-center gap-2">
                <div className="chiprow">
                  {FILTERS.map((f) => (
                    <button
                      key={f}
                      type="button"
                      aria-pressed={filter === f}
                      className={cn("chip !font-sans", filter === f && "on")}
                      onClick={() => setFilter(f)}
                    >
                      {f}
                    </button>
                  ))}
                </div>
                <span className="ml-1.5 w-[220px]">
                  <Inp
                    aria-label="Search region, city"
                    placeholder="Search region, city…"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    prefixNode={<Icon id="s-search" className="h-3.5 w-3.5 text-ink3" />}
                  />
                </span>
              </div>

              {cellRows.length > 0 ? (
                <>
                  <Eyebrow>Your cells · customer cloud</Eyebrow>
                  <div className="flex flex-col gap-2">
                    {cellRows.map((c) => {
                      const on = target.kind === "cell" && target.id === c.id;
                      const prov = c.provider.toUpperCase();
                      return (
                        <button
                          key={c.id}
                          type="button"
                          aria-pressed={on}
                          onClick={() => setTarget({ kind: "cell", id: c.id })}
                          className={cn(
                            "card flex items-center gap-3 px-3 py-2.5 text-left",
                            on && "border-steel",
                          )}
                          style={on ? { background: "var(--steel-tint)" } : undefined}
                        >
                          <span className="glyph h-6 w-6">
                            <Icon id="s-globe" className="!h-3 !w-3" />
                          </span>
                          <span className="flex-1">
                            <span className="flex items-center gap-2">
                              <b className="text-12">{c.name}</b>
                              <Pill tone="mut">customer {prov}</Pill>
                            </span>
                            <span className="mono mt-0.5 block text-10 text-ink3">
                              {c.region.replace("/", " · ")} · customer {prov} · healthy
                            </span>
                          </span>
                          <span className="mono text-10 text-ink3">
                            {CELL_DISPLAY[c.id]?.latency ?? ""}
                          </span>
                          {on ? (
                            <Icon id="s-check" className="h-[13px] w-[13px] text-steel" />
                          ) : null}
                        </button>
                      );
                    })}
                  </div>
                </>
              ) : null}

              {managedRows.length > 0 ? (
                <>
                  <Eyebrow>Managed regions · our cloud</Eyebrow>
                  <div className="flex flex-col gap-2">
                    {managedRows.map((m) => {
                      const on = target.kind === "managed" && target.id === m.id;
                      return (
                        <button
                          key={m.id}
                          type="button"
                          aria-pressed={on}
                          onClick={() => setTarget({ kind: "managed", id: m.id })}
                          className={cn(
                            "card flex items-center gap-3 px-3 py-2.5 text-left",
                            on && "border-steel",
                          )}
                          style={on ? { background: "var(--steel-tint)" } : undefined}
                        >
                          <span className="glyph h-6 w-6">
                            <Icon id="s-globe" className="!h-3 !w-3" />
                          </span>
                          <span className="flex-1">
                            <span className="flex items-center gap-2">
                              <b className={cn("text-12", on && "text-steel")}>{m.title}</b>
                              <Pill tone="mut">managed</Pill>
                            </span>
                            <span className="mono mt-0.5 block text-10 text-ink3">{m.sub}</span>
                          </span>
                          {on ? (
                            <Icon id="s-check" className="h-[13px] w-[13px] text-steel" />
                          ) : null}
                        </button>
                      );
                    })}
                  </div>
                </>
              ) : null}

              <div className="flex items-center gap-2 text-11">
                <Pill tone="ok">
                  permitted — policy <span className="mono">allowed-regions</span> expanded by asha
                  for the US launch
                </Pill>
                <span className="mono text-10 text-ink3">evt_44be… · yesterday</span>
              </div>
              <p className="border-hair border-t pt-2.5 text-10 leading-relaxed text-ink3">
                One combined list, distance-sorted, filterable — a customer cell is just a region
                whose hardware you own. Policy <span className="mono">allowed-regions</span> can
                restrict this list (a restricted target shows disabled with the policy named),
                exactly as it does for managed regions.
              </p>
            </Card>

            <Card className="flex flex-col gap-2.5 p-4">
              <Flabel>Shape</Flabel>
              <div className="grid grid-cols-2 gap-2.5">
                <button
                  type="button"
                  aria-pressed={shape === "clone"}
                  onClick={() => setShape("clone")}
                  className={cn(
                    "card flex flex-col gap-1.5 p-3 text-left",
                    shape === "clone" && "border-steel",
                  )}
                  style={shape === "clone" ? { background: "var(--steel-tint)" } : undefined}
                >
                  <b className={cn("text-12p5", shape === "clone" && "text-steel")}>
                    Clone production's shape
                  </b>
                  <span className="text-10p5 leading-relaxed text-ink2">
                    All 7 services, bindings re-wired inside the new region, scaled to{" "}
                    <span className="mono">S</span> by default — promote to full scale when the
                    client goes live.
                  </span>
                </button>
                <button
                  type="button"
                  aria-pressed={shape === "empty"}
                  onClick={() => setShape("empty")}
                  className={cn(
                    "card flex flex-col gap-1.5 p-3 text-left",
                    shape === "empty" && "border-steel",
                  )}
                  style={shape === "empty" ? { background: "var(--steel-tint)" } : undefined}
                >
                  <b className={cn("text-12p5", shape === "empty" && "text-steel")}>Start empty</b>
                  <span className="text-10p5 leading-relaxed text-ink2">
                    Add services one by one — the environment exists as a boundary first.
                  </span>
                </button>
              </div>
              <div>
                <Flabel>Data</Flabel>
                <div className="chiprow">
                  <button
                    type="button"
                    aria-pressed={data === "empty"}
                    className={cn("chip !font-sans", data === "empty" && "on")}
                    onClick={() => setData("empty")}
                  >
                    Empty schema + migrations
                  </button>
                  <button
                    type="button"
                    aria-pressed={data === "branch"}
                    className={cn("chip !font-sans", data === "branch" && "on")}
                    onClick={() => setData("branch")}
                  >
                    Branch from production
                  </button>
                </div>
                <p className="mt-2 text-10p5 leading-relaxed text-ink3">
                  Branching across regions works — initial copy is a one-time{" "}
                  <span className="mono">21.4 GB · ~$0.43</span> egress, then deltas. For a US
                  client, an empty schema usually respects data residency better.
                </p>
              </div>
            </Card>

            <div className="flex items-center gap-3.5 text-11">
              <Pill tone="ok">no cross-region bindings — everything co-located ✓</Pill>
              <span className="text-ink3">
                this is why the stack moves, not one instance: no torn topology, no egress tax on
                every query
              </span>
            </div>

            <Card className="flex items-center gap-3 px-4 py-3">
              <Dot tone="ok" />
              <span className="flex-1 text-11 text-ink2">
                After the target, everything is the same: services clone at their shapes, bindings
                re-wire, the estimate builds. <b>The developer never learns a second workflow.</b>
              </span>
            </Card>
          </div>

          <div className="flex w-[310px] shrink-0 flex-col gap-3">
            {targetCell ? (
              <Card className="flex flex-col gap-2 border-steel/40 p-4">
                <Eyebrow className="text-steel">Estimate · into {targetCell.name}</Eyebrow>
                <div className="mono text-[22px] font-semibold tracking-[-0.5px]">
                  {totalLabel}
                  <span className="ml-1 text-11 font-normal text-ink3">/mo</span>
                </div>
                <p className="text-10p5 leading-relaxed text-ink2">
                  {shape === "clone" ? "Clone of production at S scale. " : ""}On a customer cell
                  you pay Steloit's <b>control-plane fee</b>; compute &amp; storage bill to{" "}
                  <b>your</b> cloud account, not ours.
                </p>
                <div className="flex flex-col gap-1 border-hair border-t pt-2 text-10 text-ink3">
                  <div className="flex justify-between">
                    <span>Steloit control plane</span>
                    <span className="mono">{totalLabel}/mo</span>
                  </div>
                  <div className="flex justify-between">
                    <span>{targetCell.provider.toUpperCase()} infra (your account)</span>
                    <span className="mono">~$180/mo est</span>
                  </div>
                </div>
              </Card>
            ) : (
              <Card className="flex flex-col gap-2 p-4">
                <Eyebrow>Estimated monthly cost</Eyebrow>
                <div className="mono text-[28px] font-semibold tracking-[-0.5px]">
                  {totalLabel}
                  <span className="ml-1.5 text-11 font-normal text-ink3">
                    {shape === "clone"
                      ? "/month · cloned shape at S"
                      : "/month · empty — services priced as you add them"}
                  </span>
                </div>
                {shape === "clone" ? (
                  <div className="mt-1 flex flex-col gap-1.5">
                    {[
                      { label: "db-main + db-reports · S", amount: "$21" },
                      { label: "api + worker · S", amount: "$17" },
                      { label: "cache, jobs, assets · S", amount: "$8" },
                    ].map((line) => (
                      <div key={line.label} className="flex justify-between text-11p5">
                        <span className="text-ink2">{line.label}</span>
                        <span className="mono">{line.amount}</span>
                      </div>
                    ))}
                  </div>
                ) : null}
                <p className="mt-1 border-hair border-t pt-2.5 text-10p5 leading-relaxed text-ink3">
                  Rolls up as its own line —{" "}
                  <b className="text-ink2">per-client cost is free accounting</b> when the client is
                  an environment (or project).
                </p>
              </Card>
            )}
            {targetCell ? (
              <Card className="flex flex-col gap-1.5 p-4">
                <Eyebrow>Inherited, unchanged</Eyebrow>
                <div className="flex flex-col gap-1.5 text-10p5 text-ink2">
                  <div>
                    Private networking &amp; IP allowlists → <span className="mono">Network</span>{" "}
                    on the cell
                  </div>
                  <div>Observe, Events, Deploy → identical</div>
                  <div>Support access → audit-logged, time-boxed</div>
                </div>
              </Card>
            ) : null}
            <Btn
              variant="p"
              className="h-[38px] justify-center"
              onClick={submit}
              disabled={createEnv.isPending || !name.trim()}
            >
              {buttonLabel}
            </Btn>
            <Btn
              variant="s"
              className="justify-center"
              onClick={() =>
                navigate({
                  to: "/$org/$project",
                  params: { org, project },
                  search: { env: "production" },
                })
              }
            >
              Cancel
            </Btn>
            <div className="flex justify-center">
              <Copybit>{cli}</Copybit>
            </div>
            <Card className="p-3 text-10p5 leading-relaxed text-ink3">
              Region hierarchy:{" "}
              <b className="text-ink2">
                org = prefill · environment = home · instance = inherit, exception by C7
              </b>
              . A whole client deployment is an environment — or its own project, if it needs
              separate members and billing.
            </Card>
          </div>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/new-env")({
  component: NewEnvironmentPage,
});
