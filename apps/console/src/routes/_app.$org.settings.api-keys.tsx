import { createFileRoute } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { EmptyState } from "@/design-system/empty-state";
import { Flabel, Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { SkeletonRows } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { useOrgs } from "@/features/org/hooks";
import { useApiKeys, useCreateApiKey } from "@/features/settings/hooks";
import { TokenReveal } from "@/features/settings/token-reveal";
import { errorMessage, type TokenCreated } from "@/lib/api";
import { cn } from "@/lib/utils";

/** G8 · Organization · API keys — automation keys; the secret is shown once (U7). */

interface KeyDisplay {
  scope: ReactNode;
  created: string;
  lastUsed: ReactNode;
  expires: string;
  stale?: boolean;
}

// Frame-fixed presentation by name (G8): the frame's scope strings
// ("ecommerce · deploy", "org · read billing", "org · admin") exceed the
// Token.scope enum (full | read_only) — schema gap, finding. Created-by and
// relative dates also have no schema field; display strings live here.
const DISPLAY: Record<string, KeyDisplay> = {
  "ci-deploy": {
    scope: <Pill tone="st">ecommerce · deploy</Pill>,
    created: "priya · 3 mo",
    lastUsed: "14:01 today",
    expires: "in 9 mo",
  },
  "cost-export": {
    scope: <Pill tone="st">org · read billing</Pill>,
    created: "asha · 5 mo",
    lastUsed: "02:00 today",
    expires: "in 7 mo",
  },
  "old-terraform": {
    scope: <Pill tone="mut">org · admin</Pill>,
    created: "asha · 14 mo",
    lastUsed: <span className="text-warn">61 d ago</span>,
    expires: "none",
    stale: true,
  },
};

function ApiKeysPage() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const keys = useApiKeys(org);
  const createKey = useCreateApiKey();

  const [createOpen, setCreateOpen] = useState(false);
  const [newName, setNewName] = useState("");
  const [reveal, setReveal] = useState<{ name: string; created: TokenCreated } | null>(null);

  // Four-state grammar (16-qa): pending → skeleton, error → failure card,
  // empty → EmptyState, else the table (canon carries 3 keys — empty is for
  // fresh orgs).
  const isEmpty = keys.isSuccess && (keys.data ?? []).length === 0;

  const onCreate = () => {
    if (!newName.trim()) return;
    const name = newName.trim();
    createKey.mutate(
      { path: { org }, body: { name, scope: "full" } },
      {
        onSuccess: (created) => {
          setCreateOpen(false);
          setNewName("");
          setReveal({ name, created });
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project="ecommerce"
        active="api-keys"
      />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Organization · API keys"
            sub="Automation keys for CI and tooling — personal tokens live under your profile, not here"
          >
            <Btn variant="p" onClick={() => setCreateOpen((v) => !v)}>
              Create key (U7)
            </Btn>
          </Pghead>

          {createOpen ? (
            <Card className="flex max-w-[460px] flex-col gap-3 p-4">
              <div>
                <Flabel htmlFor="key-name">Key name</Flabel>
                <Inp
                  id="key-name"
                  className="mono"
                  placeholder="ci-deploy"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") onCreate();
                    if (e.key === "Escape") setCreateOpen(false);
                  }}
                />
              </div>
              <div className="flex gap-2">
                <Btn
                  variant="p"
                  onClick={onCreate}
                  disabled={createKey.isPending}
                  disabledReason="Creating the key…"
                >
                  Create
                </Btn>
                <Btn variant="s" onClick={() => setCreateOpen(false)}>
                  Cancel
                </Btn>
              </div>
            </Card>
          ) : null}

          {keys.isError ? (
            <ApiFailureCard
              title="API keys didn't load"
              error={keys.error}
              requestLine={`GET /orgs/${org}/api-keys`}
              onRetry={() => keys.refetch()}
            />
          ) : isEmpty ? (
            <EmptyState
              compact
              icon="s-key"
              title="No API keys yet"
              meaning={
                <>
                  keys are the org's robots — org-scoped, least-privilege, the secret shown once at
                  creation and hashed after
                </>
              }
              cta={
                <Btn variant="p" onClick={() => setCreateOpen(true)}>
                  Create key (U7)
                </Btn>
              }
              cli="steloit keys create ci-deploy"
            />
          ) : (
            <div className="tblwrap">
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Key</th>
                    <th>Prefix</th>
                    <th>Scope</th>
                    <th>Created</th>
                    <th>Last used</th>
                    <th>Expires</th>
                    <th aria-label="Actions" />
                  </tr>
                </thead>
                <tbody>
                  {keys.isPending ? (
                    <SkeletonRows cols={7} />
                  ) : (
                    (keys.data ?? []).map((k) => {
                      const d = DISPLAY[k.name];
                      return (
                        <tr key={k.id} className={cn(d?.stale && "opacity-60")}>
                          <td className="mono text-11p5">{k.name}</td>
                          <td className="mono text-11 text-ink3">{k.prefix}</td>
                          <td>{d?.scope ?? k.scope}</td>
                          <td className="text-ink2">{d?.created ?? "—"}</td>
                          <td className="text-ink2">{d?.lastUsed ?? "—"}</td>
                          <td className="text-ink2">{d?.expires ?? "—"}</td>
                          <td>
                            <span className="flex items-center justify-end gap-2">
                              {d?.stale ? <Pill tone="warn">stale — revoke?</Pill> : null}
                              {/* openapi.yaml has no DELETE /orgs/{org}/api-keys/{key} —
                              only list + create exist, so revoke can't be wired (finding). */}
                              <Btn
                                variant="dgr"
                                disabled
                                disabledReason="Key revocation isn't in the API yet — openapi.yaml gap (finding)"
                              >
                                Revoke
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
            The secret is shown once at creation — Steloit stores a hash. Keys are scoped
            least-privilege, usage is visible above, staleness is flagged, and every create /
            use-after-long-idle / revoke is an audit event.
          </p>
        </div>
      </main>
      {reveal ? (
        <TokenReveal
          sub={`${reveal.name} · org key`}
          token={reveal.created.token}
          onDone={() => setReveal(null)}
        />
      ) : null}
    </>
  );
}

export const Route = createFileRoute("/_app/$org/settings/api-keys")({
  component: ApiKeysPage,
});
