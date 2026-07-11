import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { NitDisabled, NitLink, Snav } from "@/app/shell/snav";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Icon } from "@/design-system/icon";
import { Inp } from "@/design-system/inp";
import { Pill, type PillTone } from "@/design-system/pill";
import { useOrgs } from "@/features/org/hooks";
import { useTemplatesList } from "@/features/settings/hooks";
import { fmtMoney } from "@/lib/fmt";
import { cn } from "@/lib/utils";

/**
 * C5 · Templates gallery — the delivery vehicle for best practices. The six
 * gallery cards are frame-fixed (C5 copy verbatim); fixtures define only the
 * 3 org templates (store-baseline, api-worker-pair, analytics-starter), which
 * merge into the same grid from useTemplatesList. File is un-nested
 * (`new-project_.`) because W2's route renders no Outlet — same URL, flat route.
 */

type Category = "Storefront" | "SaaS" | "AI" | "Data" | "Internal tools";

interface GalleryCard {
  name: string;
  pill?: { tone: PillTone; label: string };
  deploys: string;
  desc: string;
  services: string[];
  est: string;
  category?: Category;
  featured?: boolean;
}

/** The six gallery cards — frame-fixed (C5), microcopy verbatim. */
const GALLERY: GalleryCard[] = [
  {
    name: "store",
    pill: { tone: "st", label: "featured" },
    deploys: "1.2k deploys",
    desc: "Full commerce stack: catalog, carts, uploads, order jobs — the ecommerce project in this gallery came from it.",
    services: ["Postgres", "Valkey", "Storage", "Queue", "api + worker"],
    est: "est. $184/mo",
    category: "Storefront",
    featured: true,
  },
  {
    name: "saas-starter",
    pill: { tone: "mut", label: "by Steloit" },
    deploys: "940 deploys",
    desc: "Multi-tenant SaaS core with auth integrated (not rebuilt) per the boundary rules.",
    services: ["Postgres", "Valkey", "api", "Clerk pattern"],
    est: "est. $96/mo",
    category: "SaaS",
  },
  {
    name: "ai-app",
    pill: { tone: "ai", label: "pgvector" },
    deploys: "1.6k deploys",
    desc: "RAG and embeddings on the anchor database — no separate vector product to learn.",
    services: ["Postgres + pgvector", "Queue", "api + worker"],
    est: "est. $74/mo",
    category: "AI",
  },
  {
    name: "analytics-etl",
    pill: { tone: "mut", label: "by Steloit" },
    deploys: "410 deploys",
    desc: "Scheduled ingestion into a reporting database; dashboards read-only via binding.",
    services: ["Postgres", "Queue", "worker + cron"],
    est: "est. $88/mo",
    category: "Data",
  },
  {
    name: "queue-worker",
    pill: { tone: "mut", label: "minimal" },
    deploys: "780 deploys",
    desc: "The smallest honest background-job setup — queue, worker, DLQ, retry policy.",
    services: ["Queue", "worker"],
    est: "est. $16/mo",
  },
  {
    name: "docs-site",
    pill: { tone: "mut", label: "static" },
    deploys: "520 deploys",
    desc: "Static docs with preview environments per PR — no database at all. Rail shows one icon.",
    services: ["Web service", "Storage"],
    est: "est. $14/mo",
  },
];

const CATEGORIES: ("All" | Category)[] = [
  "All",
  "Storefront",
  "SaaS",
  "AI",
  "Data",
  "Internal tools",
];

function TemplatesGallery() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const orgName = orgRecord?.name ?? org;
  const templates = useTemplatesList(org);

  const [category, setCategory] = useState<"All" | Category>("All");
  const [search, setSearch] = useState("");
  const q = search.trim().toLowerCase();

  const gallery = GALLERY.filter(
    (g) => (category === "All" || g.category === category) && g.name.toLowerCase().includes(q),
  );
  // Org templates carry no gallery category — they surface under All (and search).
  const orgTemplates = (templates.data ?? []).filter(
    (t) => category === "All" && t.name.toLowerCase().includes(q),
  );
  const empty = gallery.length === 0 && orgTemplates.length === 0;

  return (
    <>
      <Snav>
        <div className="snhead">
          <span
            className="cav h-6 w-6 rounded-md"
            style={{ background: "linear-gradient(135deg,#E36C4B,#B34A2E)" }}
          >
            {orgName.slice(0, 1).toUpperCase()}
          </span>
          <div>
            <div className="t">{orgName}</div>
            <div className="u">New project</div>
          </div>
        </div>
        <NitLink to="/$org/new-project" params={{ org }} icon="s-plus" label="Describe your app" />
        <NitLink
          to="/$org/new-project/templates"
          params={{ org }}
          icon="s-grid"
          label="From a template"
          on
        />
        <NitDisabled icon="s-term" label="From the CLI" reason="CLI docs — steloit init" />
      </Snav>
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Start from a template"
            sub="Services, bindings, example code and sensible defaults — pre-wired, fully editable before anything exists"
          >
            <span className="w-[240px]">
              <Inp
                aria-label="Search templates"
                placeholder="Search templates…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                prefixNode={<Icon id="s-search" className="h-3.5 w-3.5 text-ink3" />}
              />
            </span>
          </Pghead>

          <div className="chiprow">
            {CATEGORIES.map((c) => (
              <button
                key={c}
                type="button"
                aria-pressed={category === c}
                className={cn("chip !font-sans", category === c && "on")}
                onClick={() => setCategory(c)}
              >
                {c}
              </button>
            ))}
          </div>

          {empty ? (
            <Card dashed className="flex flex-col items-center gap-1 p-8 text-center">
              <span className="text-12p5 font-medium text-ink2">
                No templates {q ? `match “${search.trim()}”` : "in this category yet"}
              </span>
              <span className="text-10p5 text-ink3">
                Community templates: curated submissions open at v2.
              </span>
            </Card>
          ) : (
            <div className="grid grid-cols-3 gap-3">
              {gallery.map((g) => (
                <Card
                  key={g.name}
                  className={cn("flex flex-col gap-2.5 p-4", g.featured && "border-steel/45")}
                >
                  <div className="flex items-center gap-2">
                    <b className="text-13">{g.name}</b>
                    {g.pill ? <Pill tone={g.pill.tone}>{g.pill.label}</Pill> : null}
                    <span className="sp" />
                    <span className="mono text-10 text-ink3">{g.deploys}</span>
                  </div>
                  <div className="text-11 leading-relaxed text-ink2">{g.desc}</div>
                  <div className="flex flex-wrap gap-1.5">
                    {g.services.map((s) => (
                      <Pill key={s} tone="mut">
                        {s}
                      </Pill>
                    ))}
                  </div>
                  <div className="mt-auto flex items-center">
                    <span className="mono text-11 text-ink2">{g.est}</span>
                    <span className="sp" />
                    <Link to="/$org/template/$tpl" params={{ org, tpl: g.name }}>
                      <Btn variant="gh" className="text-steel">
                        Preview →
                      </Btn>
                    </Link>
                  </div>
                </Card>
              ))}
              {orgTemplates.map((t) => (
                <Card key={t.id} className="flex flex-col gap-2.5 p-4">
                  <div className="flex items-center gap-2">
                    <b className="mono text-12p5">{t.name}</b>
                    {t.visibility === "org" ? (
                      <Pill tone="st">org</Pill>
                    ) : (
                      <Pill tone="mut">restricted</Pill>
                    )}
                    <span className="sp" />
                    <span className="mono text-10 text-ink3">used {t.used_by_count ?? 0}×</span>
                  </div>
                  <div className="text-11 leading-relaxed text-ink2">
                    Saved shape · v{t.version}
                    {typeof t.contents?.services === "number"
                      ? ` · ${t.contents.services} services`
                      : ""}{" "}
                    — an org asset, managed under Settings → Templates.
                  </div>
                  <div className="mt-auto flex items-center">
                    <span className="mono text-11 text-ink2">
                      est. {fmtMoney(t.monthly_estimate_cents ?? 0)}/mo
                    </span>
                    <span className="sp" />
                    <Link to="/$org/template/$tpl" params={{ org, tpl: t.name }}>
                      <Btn variant="gh" className="text-steel">
                        Preview →
                      </Btn>
                    </Link>
                  </div>
                </Card>
              ))}
            </div>
          )}

          <div className="flex items-center gap-2 text-10p5 text-ink3">
            <Icon id="s-term" className="h-3 w-3" />
            Every template is a CLI command too:
            <Copybit>steloit init --template store</Copybit>
            <span className="sp" />
            <span>
              Community templates: curated submissions open at v2. Your saved templates are managed
              under{" "}
              <Link
                to="/$org/settings/templates"
                params={{ org }}
                className="font-medium text-steel"
              >
                Settings → Templates (T1)
              </Link>
              .
            </span>
          </div>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/new-project_/templates")({
  component: TemplatesGallery,
});
