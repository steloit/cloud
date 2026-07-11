import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { EmptyRow, EmptyState } from "@/design-system/empty-state";
import { Icon } from "@/design-system/icon";
import { Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { SkeletonRows } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { useOrgs } from "@/features/org/hooks";
import { useTemplatesList } from "@/features/settings/hooks";
import { fmtMoney } from "@/lib/fmt";
import { cn } from "@/lib/utils";

/** T1 · Organization · Templates — saved shapes; editing never touches what was created. */

interface TemplateDisplay {
  sub?: string;
  contents: string;
  used: string;
  updated: string;
}

// Frame-fixed presentation by name (T1): Template.contents is an opaque object and
// the schema has no per-service breakdown, usage string, or "Mon d · actor" stamp —
// display strings live here where the schema lacks them.
const DISPLAY: Record<string, TemplateDisplay> = {
  "store-baseline": {
    sub: "saved from ecommerce / production · v4",
    contents: "7 services — db×2, cache, assets, jobs, api, worker",
    used: "2×",
    updated: "Mar 12 · asha",
  },
  "api-worker-pair": {
    contents: "api + worker + jobs, pre-wired bindings",
    used: "4×",
    updated: "Feb 28 · priya",
  },
  "analytics-starter": {
    contents: "db + worker + bucket, pgvector on",
    used: "1×",
    updated: "Feb 9 · asha",
  },
};

type Filter = "all" | "org" | "restricted";

const EDIT_REASON = "Template editing lands in Phase 4";

function TemplatesPage() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const templates = useTemplatesList(org);

  const [filter, setFilter] = useState<Filter>("all");
  const [search, setSearch] = useState("");

  const rows = (templates.data ?? []).filter(
    (t) =>
      (filter === "all" || t.visibility === filter) &&
      t.name.toLowerCase().includes(search.trim().toLowerCase()),
  );

  // Four-state grammar (16-qa): pending → skeleton, error → failure card,
  // truly empty (no templates at all) → EmptyState, filtered-to-nothing →
  // EmptyRow inside the table, else rows. Canon carries 3 templates.
  const isEmpty = templates.isSuccess && (templates.data ?? []).length === 0;

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project="ecommerce"
        active="templates"
      />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Organization · Templates"
            sub="Your saved shapes — org assets, managed here, consumed anywhere you create (C1, C5, onboarding, CLI). Editing a template never touches anything created from it."
          >
            <Link to="/$org/new-project/templates" params={{ org }}>
              <Btn variant="s">Official gallery (C5) ↗</Btn>
            </Link>
            <Link to="/$org/settings/templates/new" params={{ org }}>
              <Btn variant="p">＋ New template</Btn>
            </Link>
          </Pghead>

          <div className="flex items-center gap-2">
            <div className="chiprow">
              <button
                type="button"
                className={cn("chip", filter === "all" && "on")}
                onClick={() => setFilter("all")}
              >
                All · 3
              </button>
              <button
                type="button"
                className={cn("chip", filter === "org" && "on")}
                onClick={() => setFilter("org")}
              >
                Org-visible
              </button>
              <button
                type="button"
                className={cn("chip", filter === "restricted" && "on")}
                onClick={() => setFilter("restricted")}
              >
                Restricted
              </button>
            </div>
            <span className="ml-auto w-[220px]">
              <Inp
                aria-label="Search templates"
                placeholder="Search…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                prefixNode={<Icon id="s-search" className="h-3.5 w-3.5 text-ink3" />}
              />
            </span>
          </div>

          {templates.isError ? (
            <ApiFailureCard
              title="Templates didn't load"
              error={templates.error}
              requestLine={`GET /orgs/${org}/templates`}
              onRetry={() => templates.refetch()}
            />
          ) : isEmpty ? (
            <EmptyState
              compact
              icon="s-doc"
              title="No templates yet"
              meaning={
                <>
                  capture a project's shape once, instantiate it anywhere — secrets are never
                  inside; bindings re-mint per consumer at create
                </>
              }
              cta={
                <Link to="/$org/settings/templates/new" params={{ org }}>
                  <Btn variant="p">＋ New template</Btn>
                </Link>
              }
              cli="steloit template save ecommerce/production --name store-baseline"
            />
          ) : (
            <div className="tblwrap">
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Template</th>
                    <th>Contents</th>
                    <th>Est. / mo</th>
                    <th>Visibility</th>
                    <th>Used</th>
                    <th>Updated</th>
                    <th aria-label="Actions" />
                  </tr>
                </thead>
                <tbody>
                  {templates.isPending ? (
                    <SkeletonRows cols={7} />
                  ) : rows.length === 0 ? (
                    <EmptyRow cols={7}>
                      No templates match — clear the search or switch the visibility filter
                    </EmptyRow>
                  ) : (
                    rows.map((t) => {
                      const d = DISPLAY[t.name];
                      return (
                        <tr key={t.id}>
                          <td>
                            <span className="mono text-11p5">{t.name}</span>
                            {d?.sub ? (
                              <div className="mt-0.5 text-10p5 text-ink3">{d.sub}</div>
                            ) : null}
                          </td>
                          <td className="text-ink2">{d?.contents ?? "—"}</td>
                          <td className="mono">{fmtMoney(t.monthly_estimate_cents ?? 0)}</td>
                          <td>
                            {t.visibility === "org" ? (
                              <Pill tone="st">org</Pill>
                            ) : (
                              <Pill tone="mut">restricted</Pill>
                            )}
                          </td>
                          <td className="text-ink2">{d?.used ?? "—"}</td>
                          <td className="text-ink2">{d?.updated ?? "—"}</td>
                          <td>
                            <span className="flex items-center justify-end gap-1.5">
                              <Btn variant="gh" disabled disabledReason={EDIT_REASON}>
                                Edit
                              </Btn>
                              <Btn variant="gh" disabled disabledReason={EDIT_REASON}>
                                Duplicate
                              </Btn>
                              <Btn variant="gh" disabled disabledReason={EDIT_REASON}>
                                Delete…
                              </Btn>
                            </span>
                          </td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>
          )}

          <p className="text-11 text-ink3">
            Secrets are never inside a template. Bindings are captured as placeholders and re-minted
            per consumer at create — same rule as everywhere. Official templates (store,
            internal-tools…) live in the gallery and are managed by Steloit.
          </p>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/settings/templates")({
  component: TemplatesPage,
});
