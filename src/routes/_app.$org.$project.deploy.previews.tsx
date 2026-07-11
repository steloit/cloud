import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Dot, Pill } from "@/design-system/pill";
import { SkeletonLines, SkeletonRows } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { useEnvironments } from "@/features/projects/hooks";
import { fmtMoneyPerMonth } from "@/lib/fmt";

/**
 * DP3 · Deploy · Previews — a preview is an environment. pr-142's cost comes
 * from the environments fixture (env_pr142); the torn-down rows and PR-bot
 * card are frame-fixed DP3 canon (Git integration lands in Phase 3).
 */
function PreviewsPage() {
  const { project } = Route.useParams();
  const environments = useEnvironments(project);
  const pr142 = environments.data?.find((e) => e.name === "preview/pr-142");
  const pr142Cost =
    pr142?.monthly_cost_cents !== undefined ? fmtMoneyPerMonth(pr142.monthly_cost_cents) : "…";

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Deploy · Previews"
          sub="A preview is an environment — created when a PR opens, commented on the PR, expired by policy. Same grammar as production, S-sized shape"
        >
          {/* Static context chip (frame microcopy) — a policy picker needs
              the policy endpoints wired to previews (finding). */}
          <span className="chip">policy: preview-minimal v2</span>
        </Pghead>

        {/* Four-state grammar (16-qa) on the environments query. The torn-down
            rows below are frame-fixed history and stay; the live pr-142 card
            and its table row are data-backed, so they render only when the env
            list actually carries the preview — an empty env list gets an
            honest line instead of a hero card claiming a preview exists. */}
        {environments.isError ? (
          <ApiFailureCard
            title="Previews didn't load"
            error={environments.error}
            requestLine={`GET /projects/${project}/envs`}
            onRetry={() => environments.refetch()}
          />
        ) : environments.isPending ? (
          <>
            <Card className="p-4">
              <SkeletonLines lines={4} />
            </Card>
            <div className="tblwrap">
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Preview</th>
                    <th>PR</th>
                    <th>Author</th>
                    <th>Status</th>
                    <th>Cost</th>
                    <th>Lifecycle</th>
                  </tr>
                </thead>
                <tbody>
                  <SkeletonRows cols={6} />
                </tbody>
              </table>
            </div>
          </>
        ) : (
          <>
            {pr142 ? (
              <Card className="flex flex-col gap-3 p-4">
                <div className="flex items-center gap-2.5">
                  <Dot tone="ok" />
                  <b className="text-13">preview / pr-142</b>
                  <Pill tone="ok">open</Pill>
                  <Pill tone="warn">branch flagged · branch-data-masking</Pill>
                  <span className="mono ml-auto text-11p5">{pr142Cost} · expires in 5 d</span>
                </div>
                <div className="text-11p5 text-ink3">
                  acme/store #142 "gift card refunds" · marco · updated 2 h ago · deploys on every
                  push
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Pill tone="ok">api</Pill>
                  <Pill tone="ok">admin</Pill>
                  <Pill tone="ok">db · branch of production</Pill>
                  <Pill tone="ok">cache</Pill>
                  <Pill tone="mut">jobs · excluded by policy</Pill>
                  <Pill tone="mut">worker · excluded</Pill>
                  <Pill tone="st">masking job · system</Pill>
                </div>
                <div className="flex items-center gap-2.5">
                  <Copybit>https://pr-142.ecommerce.previews.steloit.app</Copybit>
                  {/* The preview host is a canon fixture — nothing live to
                      open; a live Open lands with real preview deploys. */}
                  <Btn
                    variant="s"
                    disabled
                    disabledReason="The preview URL is canon fixture — there's no live host to open (finding)"
                  >
                    Open
                  </Btn>
                  <Btn
                    variant="s"
                    disabled
                    disabledReason="Redeploy lands with the Git integration — previews deploy on PR push (finding)"
                  >
                    Redeploy
                  </Btn>
                  <Btn
                    variant="dgr"
                    disabled
                    disabledReason="Teardown lands with an environment-delete endpoint the API lacks (finding) — behind the U6 typed-confirm"
                  >
                    Tear down…
                  </Btn>
                </div>
                <div className="flex flex-col gap-1.5 rounded-lg border border-hair bg-surface2 p-3">
                  <Eyebrow>On the PR — the loop closes in GitHub</Eyebrow>
                  <div className="text-11 text-ink3">steloit-bot commented 2 h ago</div>
                  <div className="text-11p5">✅ Preview ready — pr-142.ecommerce.previews…</div>
                  <div className="mono text-11 text-ink3">
                    db: branch of production (masked · policy) · $0.07/day
                  </div>
                </div>
              </Card>
            ) : (
              <Card className="p-4 text-11p5 text-ink3">
                No live previews — open a PR and a preview environment appears here in ~60 s, no
                config.
              </Card>
            )}

            <div className="tblwrap">
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Preview</th>
                    <th>PR</th>
                    <th>Author</th>
                    <th>Status</th>
                    <th>Cost</th>
                    <th>Lifecycle</th>
                  </tr>
                </thead>
                <tbody>
                  {pr142 ? (
                    <tr>
                      <td className="mono">pr-142</td>
                      <td>#142 "gift card refunds"</td>
                      <td>marco</td>
                      <td>
                        <Pill tone="ok">live</Pill>
                      </td>
                      <td className="mono">{pr142Cost}</td>
                      <td className="text-ink3">expires in 5 d · renews on push</td>
                    </tr>
                  ) : null}
                  <tr className="opacity-55">
                    <td className="mono">pr-139</td>
                    <td>#139 "checkout retry"</td>
                    <td>marco</td>
                    <td>
                      <Pill tone="mut">merged → torn down</Pill>
                    </td>
                    <td className="mono">$1.10 total</td>
                    <td className="text-ink3">lived 4 d · Jul 3</td>
                  </tr>
                  <tr className="opacity-55">
                    <td className="mono">pr-131</td>
                    <td>#131 "catalog cache"</td>
                    <td>priya</td>
                    <td>
                      <Pill tone="mut">closed → torn down</Pill>
                    </td>
                    <td className="mono">$0.40 total</td>
                    <td className="text-ink3">lived 1 d · Jun 28</td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div className="flex flex-col gap-1.5 text-11 leading-relaxed text-ink3">
              <span>
                Exclusions come from preview-minimal v2 (G7) and are shown per service — never a
                silent diff from production.
              </span>
              <span>Torn-down previews keep their final cost line in Billing · Usage.</span>
              <span>New PR → preview in ~60 s, no config.</span>
            </div>
          </>
        )}
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/deploy/previews")({
  component: PreviewsPage,
});
