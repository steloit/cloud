import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Icon } from "@/design-system/icon";
import { Pill, type PillTone, Stlab } from "@/design-system/pill";
import { useServices } from "@/features/services/hooks";
import { resolveEnvKey } from "@/lib/canon-env";

/**
 * W5 · PostgreSQL Branches — the signature feature. 08-api has no branches
 * endpoint although W5 requires one (finding: spec change needed before this
 * page can go live); rows below are frame-fixed canon from the W5 frame.
 */

interface BranchRow {
  name: string;
  pill: { tone: PillTone; label: string };
  desc: string;
  cost?: string;
  status: { tone: "ok" | "warn"; label: string };
}

const BRANCHES: BranchRow[] = [
  {
    name: "main",
    pill: { tone: "st", label: "production" },
    desc: "the live database · 21.4 GB · PITR 7 days",
    status: { tone: "warn", label: "degraded" },
  },
  {
    name: "staging",
    pill: { tone: "mut", label: "env: staging" },
    desc: "refreshed nightly from main · 21.4 GB base + 218 MB delta",
    cost: "$4.10/mo",
    status: { tone: "ok", label: "ready" },
  },
  {
    name: "preview/pr-142",
    pill: { tone: "ai", label: "preview env · PR #142" },
    desc: "branched from main @ 13:52 · scoped credentials · auto-retires when PR closes",
    cost: "$0.90/mo",
    status: { tone: "ok", label: "ready" },
  },
  {
    name: "restore-drill-jul",
    pill: { tone: "ok", label: "restore verify" },
    desc: "restored from backup 06 Jul 03:00 · verification passed · retires in 20h",
    cost: "$0.20",
    status: { tone: "ok", label: "ready" },
  },
];

function BranchesPage() {
  const { project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));
  const svc = services.data?.find((s) => s.name === service || s.id === service);

  if (!svc) return <main className="main" />;
  if (svc.product !== "postgres") {
    return (
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Card className="p-4 text-12 text-ink2">This tab belongs to PostgreSQL instances.</Card>
        </div>
      </main>
    );
  }

  // Wrong-instance fix: the four rows above are db-main's W5 canon — every
  // other postgres instance shows only its own main branch, derived from the
  // instance itself, plus an honest hint about where branches appear.
  const canon = svc.name === "db-main";
  const rows: BranchRow[] = canon
    ? BRANCHES
    : [
        {
          name: "main",
          pill: { tone: "st", label: "production" },
          desc: "the live database · PITR window accruing",
          status:
            svc.status === "degraded"
              ? { tone: "warn", label: "degraded" }
              : { tone: "ok", label: "ready" },
        },
      ];

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          before={<Glyph id="s-db" />}
          title={service}
          sub={
            <span className="mono">
              PostgreSQL 16 · branches are copy-on-write — created in seconds, billed by delta
            </span>
          }
        >
          <Btn
            variant="p"
            disabled
            disabledReason="Branch creation needs the branches endpoint — spec change first (finding)"
          >
            <Icon id="s-branch" />
            New branch
          </Btn>
        </Pghead>

        <Card>
          {rows.map((b, i) => (
            <div
              key={b.name}
              className="flex items-center gap-3 border-hair border-b px-4 py-3 last:border-b-0"
              style={{ paddingLeft: i === 0 ? 16 : 34 }}
            >
              <Icon id="s-branch" className="h-3.5 w-3.5 text-ink3" />
              <b className="mono text-12p5">{b.name}</b>
              <Pill tone={b.pill.tone}>{b.pill.label}</Pill>
              <span className="flex-1 text-11p5 text-ink3">{b.desc}</span>
              {b.cost ? <span className="mono text-11p5">{b.cost}</span> : null}
              <Stlab tone={b.status.tone}>{b.status.label}</Stlab>
              <Btn variant="s" className="h-6 px-2.5 text-10p5">
                Open
              </Btn>
            </div>
          ))}
          {!canon ? (
            <div className="border-hair border-t border-dashed px-4 py-6 text-center text-11p5 text-ink3">
              Branches appear here as you create them — every branch is a full copy-on-write
              database.
            </div>
          ) : null}
        </Card>

        <div className="grid grid-cols-2 gap-3.5">
          <Card className="flex flex-col gap-2 p-4">
            <Eyebrow>Why branches</Eyebrow>
            <p className="text-12 leading-relaxed text-ink2">
              Every preview environment gets a <b>real copy of production data</b> in seconds,
              isolated and disposable. Restores are rehearsed the same way: restore into a branch,
              verify, promote — disaster recovery you can practice.
            </p>
          </Card>
          <Card className="flex flex-col gap-2.5 p-4">
            <Eyebrow>From the CLI</Eyebrow>
            <Copybit>steloit db branch create fix-index --from main</Copybit>
            <Copybit>steloit db branch promote fix-index --to staging</Copybit>
          </Card>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/branches")({
  component: BranchesPage,
});
