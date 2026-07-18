import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Banner } from "@/design-system/banner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Icon } from "@/design-system/icon";
import { Flabel, Inp } from "@/design-system/inp";
import { Dot, Pill } from "@/design-system/pill";
import { cn } from "@/lib/utils";

/**
 * C2/C3/C7/C9–C12 · The per-product type blocks — states of the ONE create
 * surface (ADR-014). Same skeleton every time: name → type block → bindings
 * → included defaults; the estimate rail reads the props-up BlockState.
 */

export type CreatableProduct = "postgres" | "valkey" | "web" | "worker";

export interface EstimateLineItem {
  label: string;
  amount: string;
}

/** Props-up state the create route feeds its estimate rail + submit from. */
export interface BlockState {
  name: string;
  lines: EstimateLineItem[];
  totalLabel: string;
  buttonLabel: string;
  cli: string;
  shape: Record<string, unknown>;
  bindings: { target: string; scope: "read_only" | "read_write" }[];
}

interface TypeBlockDef {
  /** Product selector strip label. */
  label: string;
  /** Per-frame page title. */
  h1: string;
  /** Appended to "in {org} / {env} · home region aws · ap-south-1". */
  hsubSuffix: string;
  /** Estimate-rail footer, per frame. */
  footer?: string;
}

export const TYPE_BLOCKS: Record<CreatableProduct, TypeBlockDef> = {
  postgres: {
    label: "PostgreSQL",
    h1: "New PostgreSQL instance",
    hsubSuffix: " — inherited from the environment; overrides are explicit exceptions (C7)",
  },
  valkey: {
    label: "Valkey",
    h1: "New Valkey instance",
    hsubSuffix: " · same form, different type block — learn one, know them all",
    footer: "Billing starts when ready.",
  },
  web: {
    label: "Web",
    h1: "New Web service",
    hsubSuffix: " · containers from your repo",
    footer:
      "First deploy starts on create from main — watch it in Deployments. Billing starts at ready.",
  },
  worker: {
    label: "Worker",
    h1: "New Worker",
    hsubSuffix: " · background jobs & schedules",
    footer:
      "A worker that mostly sleeps mostly costs nothing — previews inherit this and round to $0.",
  },
};

/* ---------- shared helpers ---------- */

const usd = (n: number) => (Number.isInteger(n) ? `$${n}` : `$${n.toFixed(2)}`);

interface BindingRowDef {
  target: string;
  /** Scope as spoken on the frame ("read-write", "publish", "write · scoped to uploads/"…). */
  scope?: string;
  note?: string;
  mutedPill?: string;
  on: boolean;
}

const apiScope = (scope?: string): "read_only" | "read_write" =>
  scope === "read" || scope?.startsWith("read-only") ? "read_only" : "read_write";

const initBind = (rows: BindingRowDef[]) =>
  Object.fromEntries(rows.map((r) => [r.target, r.on])) as Record<string, boolean>;

const toBindings = (rows: BindingRowDef[], checked: Record<string, boolean>) =>
  rows
    .filter((r) => checked[r.target])
    .map((r) => ({ target: r.target, scope: apiScope(r.scope) }));

/** Push the computed BlockState up once per real change (identity-safe). */
function useBlockSync(onChange: (s: BlockState) => void, state: BlockState) {
  const json = JSON.stringify(state);
  useEffect(() => {
    onChange(JSON.parse(json) as BlockState);
  }, [json, onChange]);
}

/* ---------- shared building blocks ---------- */

function NameCard({
  value,
  onChange,
  sublabel,
}: {
  value: string;
  onChange: (v: string) => void;
  sublabel?: string;
}) {
  return (
    <Card className="flex flex-col p-4">
      <Flabel htmlFor="svc-name">Instance name</Flabel>
      <Inp id="svc-name" value={value} onChange={(e) => onChange(e.target.value)} className="foc" />
      {sublabel ? <div className="mono mt-1.5 text-10p5 text-ink3">{sublabel}</div> : null}
    </Card>
  );
}

function Group({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2">
      <Eyebrow>{label}</Eyebrow>
      {children}
    </div>
  );
}

function Helper({ children }: { children: React.ReactNode }) {
  return <div className="text-10p5 leading-relaxed text-ink3">{children}</div>;
}

interface ChipOpt {
  label: string;
  muted?: boolean;
}

function Chips({
  options,
  isOn,
  onPick,
}: {
  options: ChipOpt[];
  isOn: (label: string) => boolean;
  onPick: (label: string) => void;
}) {
  return (
    <div className="chiprow">
      {options.map((o) =>
        o.muted ? (
          <span key={o.label} className="chip text-ink3">
            {o.label}
          </span>
        ) : (
          <button
            key={o.label}
            type="button"
            aria-pressed={isOn(o.label)}
            className={cn("chip", isOn(o.label) && "on")}
            onClick={() => onPick(o.label)}
          >
            {o.label}
          </button>
        ),
      )}
    </div>
  );
}

interface ChoiceOpt {
  id: string;
  title: string;
  sub: string;
  price?: string;
  subMono?: boolean;
  dimmed?: boolean;
  disabledReason?: string;
}

function ChoiceCards({
  options,
  value,
  onPick,
  cols = 3,
}: {
  options: ChoiceOpt[];
  value: string;
  onPick: (id: string) => void;
  cols?: 2 | 3;
}) {
  return (
    <div className={cn("grid gap-2.5", cols === 2 ? "grid-cols-2" : "grid-cols-3")}>
      {options.map((o) => (
        <button
          key={o.id}
          type="button"
          disabled={Boolean(o.disabledReason)}
          title={o.disabledReason}
          aria-pressed={value === o.id}
          onClick={() => onPick(o.id)}
          className={cn(
            "card flex flex-col gap-1.5 p-3 text-left",
            value === o.id && "border-steel",
            o.dimmed && "opacity-75",
          )}
        >
          <span className="text-12p5 font-semibold">{o.title}</span>
          <span className={cn("text-10p5 leading-snug text-ink3", o.subMono && "mono")}>
            {o.sub}
          </span>
          {o.price ? <span className="mono text-10p5 text-ink2">{o.price}</span> : null}
        </button>
      ))}
    </div>
  );
}

function Toggle({ on, onToggle, label }: { on: boolean; onToggle: () => void; label: string }) {
  return (
    <div className="flex items-center gap-3">
      <button
        type="button"
        role="switch"
        aria-checked={on}
        aria-label={`${label} — toggle`}
        onClick={onToggle}
        className={cn(
          "relative h-[18px] w-8 shrink-0 rounded-full transition-colors",
          on ? "bg-steel" : "bg-surface2",
        )}
      >
        <span
          className={cn(
            "absolute top-[2px] h-[14px] w-[14px] rounded-full bg-white transition-all",
            on ? "left-[16px]" : "left-[2px]",
          )}
        />
      </button>
      <span className="text-12 text-ink2">{label}</span>
    </div>
  );
}

function BindingsCard({
  rows,
  checked,
  onToggle,
}: {
  rows: BindingRowDef[];
  checked: Record<string, boolean>;
  onToggle: (target: string) => void;
}) {
  return (
    <Card className="flex flex-col p-4">
      <Eyebrow className="mb-1">Bindings</Eyebrow>
      {rows.map((r) => (
        <label
          key={r.target}
          className="flex cursor-pointer items-center gap-2.5 border-hair border-b py-2.5 last:border-b-0"
        >
          <input
            type="checkbox"
            checked={checked[r.target] ?? false}
            onChange={() => onToggle(r.target)}
            className="accent-steel"
          />
          <span className="mono text-12 font-semibold">{r.target}</span>
          {r.scope ? <span className="text-11 text-ink2">{r.scope}</span> : null}
          {r.note ? <span className="text-11 text-ink3">{r.note}</span> : null}
          {r.mutedPill ? <Pill tone="mut">{r.mutedPill}</Pill> : null}
        </label>
      ))}
    </Card>
  );
}

function IncludedDefaults({
  pills,
  note,
}: {
  pills: { label: string; muted?: boolean }[];
  note?: string;
}) {
  return (
    <Card className="flex flex-col gap-2 p-4">
      <Eyebrow>Included defaults</Eyebrow>
      <div className="flex flex-wrap gap-1.5">
        {pills.map((p) => (
          <Pill key={p.label} tone={p.muted ? "mut" : "ok"}>
            {p.label}
          </Pill>
        ))}
      </div>
      {note ? <Helper>{note}</Helper> : null}
    </Card>
  );
}

/* ---------- postgres · C2 ---------- */

const PG_VERSIONS = ["16.4", "15.8"];
const PG_EXTENSIONS = ["pgvector", "postgis", "pg_cron"];
const PG_SIZE_DEV = {
  id: "dev",
  title: "Dev",
  sub: "1 vCPU · 2 GB",
  lineSub: "1 vCPU / 2 GB",
  price: 19,
};
const PG_SIZES = [
  PG_SIZE_DEV,
  { id: "standard", title: "Standard", sub: "2 vCPU · 4 GB", lineSub: "2 vCPU / 4 GB", price: 58 },
  {
    id: "performance",
    title: "Performance",
    sub: "4 vCPU · 8 GB",
    lineSub: "4 vCPU / 8 GB",
    price: 112,
  },
];
const PG_BINDINGS: BindingRowDef[] = [
  { target: "worker", scope: "read-write", on: true },
  { target: "api", mutedPill: "not needed for reporting", on: false },
];

function PostgresBlock({ onChange }: { onChange: (s: BlockState) => void }) {
  const [name, setName] = useState("db-reports");
  const [version, setVersion] = useState("16.4");
  const [ext, setExt] = useState<Record<string, boolean>>({
    pgvector: true,
    postgis: false,
    pg_cron: false,
  });
  const [size, setSize] = useState("dev");
  const [storage, setStorage] = useState("10 GB");
  const [ha, setHa] = useState(false);
  const [bind, setBind] = useState(() => initBind(PG_BINDINGS));

  const sz = PG_SIZES.find((s) => s.id === size) ?? PG_SIZE_DEV;
  const gb = Number.parseInt(storage, 10) || 0;
  const total = sz.price + gb * 0.5 + (ha ? 19 : 0);

  useBlockSync(onChange, {
    name,
    lines: [
      { label: `${sz.title} · ${sz.lineSub}`, amount: usd(sz.price) },
      { label: `Storage · ${gb} GB`, amount: usd(gb * 0.5) },
      { label: "High availability", amount: ha ? "$19" : "—" },
    ],
    totalLabel: usd(total),
    buttonLabel: `Create ${name} — ${usd(total)}/mo est.`,
    cli: `steloit db create ${name} --size ${size} --storage ${gb}`,
    shape: {
      version,
      extensions: PG_EXTENSIONS.filter((e) => ext[e]),
      size,
      storage_gb: gb,
      ha,
    },
    bindings: toBindings(PG_BINDINGS, bind),
  });

  return (
    <>
      <NameCard value={name} onChange={setName} />
      <Card className="flex flex-col gap-4 p-4">
        <Group label="Version">
          <Chips
            options={PG_VERSIONS.map((v) => ({ label: v }))}
            isOn={(l) => l === version}
            onPick={setVersion}
          />
        </Group>
        <Group label="Extensions">
          {/* "+18 allowlisted" is a static count chip (muted → span, never a
              button) — browsing the full allowlist needs an extensions
              catalog endpoint (finding). */}
          <Chips
            options={[
              ...PG_EXTENSIONS.map((e) => ({ label: e })),
              { label: "+18 allowlisted", muted: true },
            ]}
            isOn={(l) => Boolean(ext[l])}
            onPick={(l) => setExt((prev) => ({ ...prev, [l]: !prev[l] }))}
          />
        </Group>
      </Card>
      <Card className="flex flex-col gap-4 p-4">
        <Group label="Size">
          <ChoiceCards
            options={PG_SIZES.map((s) => ({
              id: s.id,
              title: s.title,
              sub: s.sub,
              price: `${usd(s.price)}/mo`,
            }))}
            value={size}
            onPick={setSize}
          />
        </Group>
        <Group label="Storage">
          <div className="flex items-center gap-2.5">
            <Inp
              value={storage}
              onChange={(e) => setStorage(e.target.value)}
              className="max-w-[140px]"
              aria-label="Storage"
            />
            <span className="mono text-10p5 text-ink3">auto-grows · $0.50/GB</span>
          </div>
        </Group>
        <Toggle
          on={ha}
          onToggle={() => setHa((v) => !v)}
          label="High availability · standby + auto-failover · +$19/mo"
        />
      </Card>
      <BindingsCard
        rows={PG_BINDINGS}
        checked={bind}
        onToggle={(t) => setBind((prev) => ({ ...prev, [t]: !prev[t] }))}
      />
      <IncludedDefaults
        pills={[
          { label: "backups · PITR 7d ✓" },
          { label: "metrics & alerts ✓" },
          { label: "private network ✓" },
        ]}
        note={`included, not add-ons — the "no additional setup" promise is kept at provisioning time`}
      />
    </>
  );
}

/* ---------- valkey · C3 ---------- */

const VK_MODE_CACHE = {
  id: "cache",
  title: "Cache",
  sub: "In-memory · LRU eviction · data may be evicted",
};
const VK_MODES = [
  VK_MODE_CACHE,
  { id: "durable", title: "Durable", sub: "AOF persistence · survives restarts" },
  { id: "streams", title: "Streams", sub: "Consumer groups · event patterns" },
];
const VK_MEM_1GB = { label: "1 GB · $22", mem: "1 GB", cli: "1gb", price: 22 };
const VK_MEMORY = [
  { label: "256 MB · $8", mem: "256 MB", cli: "256mb", price: 8 },
  VK_MEM_1GB,
  { label: "4 GB · $64", mem: "4 GB", cli: "4gb", price: 64 },
];
const VK_BINDINGS: BindingRowDef[] = [
  { target: "api", scope: "read-write", note: "· VALKEY_URL injected on next deploy", on: true },
];

function ValkeyBlock({ onChange }: { onChange: (s: BlockState) => void }) {
  const [name, setName] = useState("sessions");
  const [mode, setMode] = useState("cache");
  const [memory, setMemory] = useState("1 GB · $22");
  const [eviction, setEviction] = useState("allkeys-lru");
  const [bind, setBind] = useState(() => initBind(VK_BINDINGS));

  const md = VK_MODES.find((m) => m.id === mode) ?? VK_MODE_CACHE;
  const mem = VK_MEMORY.find((m) => m.label === memory) ?? VK_MEM_1GB;

  useBlockSync(onChange, {
    name,
    lines: [{ label: `${md.title} mode · ${mem.mem}`, amount: usd(mem.price) }],
    totalLabel: usd(mem.price),
    buttonLabel: `Create ${name} — ${usd(mem.price)}/mo est.`,
    cli: `steloit valkey create ${name} --mode ${mode} --memory ${mem.cli}`,
    shape: { mode, memory: mem.mem, eviction },
    bindings: toBindings(VK_BINDINGS, bind),
  });

  return (
    <>
      <NameCard value={name} onChange={setName} />
      <Card className="flex flex-col gap-4 p-4">
        <Group label="Mode">
          <ChoiceCards options={VK_MODES} value={mode} onPick={setMode} />
          <Helper>an explicit configuration — durability is never implied</Helper>
        </Group>
        <Group label="Memory">
          <Chips
            options={VK_MEMORY.map((m) => ({ label: m.label }))}
            isOn={(l) => l === memory}
            onPick={setMemory}
          />
        </Group>
        <Group label="Eviction">
          <Inp
            value={eviction}
            onChange={(e) => setEviction(e.target.value)}
            className="mono max-w-[220px] text-12"
            aria-label="Eviction"
          />
        </Group>
      </Card>
      <BindingsCard
        rows={VK_BINDINGS}
        checked={bind}
        onToggle={(t) => setBind((prev) => ({ ...prev, [t]: !prev[t] }))}
      />
      <IncludedDefaults
        pills={[
          { label: "metrics · hit ratio, evictions ✓" },
          { label: "private network ✓" },
          { label: "backups n/a in cache mode — stated, not hidden", muted: true },
        ]}
      />
    </>
  );
}

/* ---------- web · C9 ---------- */

const WEB_SIZE_SMALL = { label: "0.5 vCPU · 1 GB · $14", lineSub: "0.5 vCPU · 1 GB", price: 14 };
const WEB_SIZES = [
  WEB_SIZE_SMALL,
  { label: "1 vCPU · 2 GB · $28", lineSub: "1 vCPU · 2 GB", price: 28 },
];
const WEB_BINDINGS: BindingRowDef[] = [
  { target: "db-main", scope: "read-only", note: "· admin views, no writes", on: true },
  { target: "cache", scope: "read-write", on: true },
  { target: "assets", on: false },
];

function WebBlock({ onChange }: { onChange: (s: BlockState) => void }) {
  const [name, setName] = useState("admin");
  const [health, setHealth] = useState("/healthz");
  const [autoDeploy, setAutoDeploy] = useState(true);
  const [size, setSize] = useState("0.5 vCPU · 1 GB · $14");
  const [bind, setBind] = useState(() => initBind(WEB_BINDINGS));

  const sz = WEB_SIZES.find((s) => s.label === size) ?? WEB_SIZE_SMALL;

  useBlockSync(onChange, {
    name,
    lines: [
      { label: `${sz.lineSub} × 1 floor`, amount: usd(sz.price) },
      { label: "Autoscale burst to 3", amount: "metered" },
    ],
    totalLabel: usd(sz.price),
    buttonLabel: `Create ${name} — ${usd(sz.price)}/mo est.`,
    cli: `steloit web create ${name} --repo acme/admin`,
    shape: {
      repo: "acme/admin",
      branch: "main",
      health_check: health,
      autodeploy: autoDeploy,
      size: sz.lineSub,
      autoscale: { floor: 1, ceiling: 3, cpu_target: 70 },
    },
    bindings: toBindings(WEB_BINDINGS, bind),
  });

  return (
    <>
      <NameCard value={name} onChange={setName} sublabel="internal back-office UI" />
      <Card className="flex flex-col gap-4 p-4">
        <Group label="Source">
          <Helper>via the org's Git integration — no per-service tokens</Helper>
          <div className="fieldrow">
            <span className="flex items-center gap-2.5">
              <span className="font-semibold text-12">acme/admin</span>
              <span className="mono text-11 text-ink3">branch main</span>
            </span>
            <Pill tone="ok">Dockerfile detected</Pill>
          </div>
        </Group>
        <Group label="Health check">
          <Inp
            value={health}
            onChange={(e) => setHealth(e.target.value)}
            className="mono max-w-[220px] text-12"
            aria-label="Health check"
          />
        </Group>
        <Toggle
          on={autoDeploy}
          onToggle={() => setAutoDeploy((v) => !v)}
          label="Auto-deploy on push · previews per PR by policy"
        />
        <Group label="Size & scaling">
          <div className="chiprow">
            {WEB_SIZES.map((s) => (
              <button
                key={s.label}
                type="button"
                aria-pressed={size === s.label}
                className={cn("chip", size === s.label && "on")}
                onClick={() => setSize(s.label)}
              >
                {s.label}
              </button>
            ))}
            {/* Static value chip — the autoscale defaults are stated, not
                editable here; a scaling editor is D13's surface. */}
            <span className="chip text-ink3">Autoscale 1–3 · CPU target 70%</span>
          </div>
        </Group>
      </Card>
      <BindingsCard
        rows={WEB_BINDINGS}
        checked={bind}
        onToggle={(t) => setBind((prev) => ({ ...prev, [t]: !prev[t] }))}
      />
      <IncludedDefaults
        pills={[
          { label: "TLS + domain ✓" },
          { label: "zero-downtime rollouts ✓" },
          { label: "structured logs → Observe ✓" },
        ]}
      />
    </>
  );
}

/* ---------- worker · C12 ---------- */

const WK_TRIGGERS: ChoiceOpt[] = [
  { id: "consume", title: "Consume a queue", sub: "emails · concurrency 4" },
  {
    id: "cron",
    title: "Cron schedule",
    sub: "e.g. 0 2 * * * — add any number later",
    subMono: true,
  },
];
const WK_BINDINGS: BindingRowDef[] = [
  { target: "emails", scope: "consume", note: "· implied by the trigger", on: true },
  { target: "db-main", scope: "read-only", on: true },
];

function WorkerBlock({ onChange }: { onChange: (s: BlockState) => void }) {
  const [name, setName] = useState("mailer");
  const [trigger, setTrigger] = useState("consume");
  const [scaleToZero, setScaleToZero] = useState(true);
  const [bind, setBind] = useState(() => initBind(WK_BINDINGS));

  useBlockSync(onChange, {
    name,
    lines: [
      { label: "Scale-to-zero floor", amount: "$3" },
      { label: "Active time · $0.02/min", amount: "metered" },
    ],
    totalLabel: "$3+",
    buttonLabel: `Create ${name} — $3/mo floor`,
    cli: `steloit worker create ${name}${trigger === "consume" ? " --consume emails" : " --cron"}${scaleToZero ? " --scale-to-zero" : ""}`,
    shape: {
      repo: "acme/store",
      command: "node workers/mailer.js",
      trigger,
      scale_to_zero: scaleToZero,
    },
    bindings: toBindings(WK_BINDINGS, bind),
  });

  return (
    <>
      <NameCard value={name} onChange={setName} sublabel="consumes the emails queue" />
      <Card className="flex flex-col gap-4 p-4">
        <Group label="Source & command">
          <div className="fieldrow">
            <span className="flex items-center gap-2.5">
              <span className="font-semibold text-12">acme/store</span>
              <span className="mono text-11 text-ink3">same repo as api · branch main</span>
            </span>
            <span className="mono text-11 text-ink2">node workers/mailer.js</span>
          </div>
        </Group>
        <Group label="Trigger">
          <ChoiceCards options={WK_TRIGGERS} value={trigger} onPick={setTrigger} cols={2} />
        </Group>
        <Toggle
          on={scaleToZero}
          onToggle={() => setScaleToZero((v) => !v)}
          label="Scale to zero · floor $3, wakes on first message in ~1 s"
        />
      </Card>
      <BindingsCard
        rows={WK_BINDINGS}
        checked={bind}
        onToggle={(t) => setBind((prev) => ({ ...prev, [t]: !prev[t] }))}
      />
    </>
  );
}

/* ---------- dispatcher ---------- */

export function TypeBlock({
  product,
  onChange,
}: {
  product: CreatableProduct;
  onChange: (s: BlockState) => void;
}) {
  switch (product) {
    case "postgres":
      return <PostgresBlock onChange={onChange} />;
    case "valkey":
      return <ValkeyBlock onChange={onChange} />;
    case "web":
      return <WebBlock onChange={onChange} />;
    case "worker":
      return <WorkerBlock onChange={onChange} />;
  }
}
