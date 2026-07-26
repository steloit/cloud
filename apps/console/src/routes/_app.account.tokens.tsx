import { createFileRoute } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { ConfirmModal } from "@/design-system/confirm";
import { EmptyState } from "@/design-system/empty-state";
import { Flabel, Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { SkeletonRows } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import {
  useCreatePersonalToken,
  usePersonalTokens,
  useRevokeToken,
} from "@/features/settings/hooks";
import { TokenReveal } from "@/features/settings/token-reveal";
import {
  errorMessage,
  listPersonalTokensQueryKey,
  queryClient,
  type TokenCreated,
} from "@/lib/api";

/** P5 · Account · Personal tokens — tokens that act as you (U7 reveal-once). */

interface TokenDisplay {
  scope: ReactNode;
  created: string;
  lastUsed: string;
  expires: string;
}

// Frame-fixed presentation by name (P5): relative dates and the frame's scope
// strings ("acts as priya · full") have no schema field — display strings live here.
const DISPLAY: Record<string, TokenDisplay> = {
  "laptop-cli": {
    scope: <Pill tone="st">acts as priya · full</Pill>,
    created: "3 mo",
    lastUsed: "14:01 today",
    expires: "in 9 mo",
  },
  "notebook-readonly": {
    scope: <Pill tone="mut">read-only · ecommerce</Pill>,
    created: "1 mo",
    lastUsed: "Jul 4",
    expires: "in 60 d",
  },
};

function TokensPage() {
  const tokens = usePersonalTokens();
  const createToken = useCreatePersonalToken();
  const revokeToken = useRevokeToken();

  const [createOpen, setCreateOpen] = useState(false);
  const [newName, setNewName] = useState("");
  const [reveal, setReveal] = useState<{ name: string; created: TokenCreated } | null>(null);
  const [revoking, setRevoking] = useState<{ id: string; name: string } | null>(null);

  // Four-state grammar (16-qa): pending → skeleton, error → failure card,
  // empty → EmptyState, else the table (canon carries 2 tokens — empty is
  // the fresh-account state).
  const isEmpty = tokens.isSuccess && (tokens.data ?? []).length === 0;

  const onCreate = () => {
    if (!newName.trim()) return;
    const name = newName.trim();
    createToken.mutate(
      { body: { name, scope: "full", expires_in_days: 90 } },
      {
        onSuccess: (created) => {
          setCreateOpen(false);
          setNewName("");
          setReveal({ name, created });
          queryClient.invalidateQueries({ queryKey: listPersonalTokensQueryKey() });
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  // Overlay census: the two-click armed button is gone — destructive verbs
  // confirm through the one ConfirmModal recipe (460).
  const onRevoke = (id: string, name: string) => {
    revokeToken.mutate(
      { path: { tok: id } },
      {
        onSuccess: () => {
          setRevoking(null);
          toast.success(`Token ${name} revoked — it stops working immediately`);
          queryClient.invalidateQueries({ queryKey: listPersonalTokensQueryKey() });
        },
        // The modal stays open on error — the verb can be retried.
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        {/* Finding: the frame shows the bare "Personal tokens" title — the design-system
            "Area · Thing" h1 grammar wins per the audit's P1 ruling. */}
        <Pghead
          title="Account · Personal tokens"
          sub="Tokens that act as you — they carry your roles and shrink the moment your roles do"
        >
          <Btn variant="p" onClick={() => setCreateOpen((v) => !v)}>
            Create token (U7)
          </Btn>
        </Pghead>

        {createOpen ? (
          <Card className="flex max-w-[460px] flex-col gap-3 p-4">
            <div>
              <Flabel htmlFor="token-name">Token name</Flabel>
              <Inp
                id="token-name"
                className="mono"
                placeholder="laptop-cli"
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
                disabled={createToken.isPending}
                disabledReason="Creating the token…"
              >
                Create
              </Btn>
              <Btn variant="s" onClick={() => setCreateOpen(false)}>
                Cancel
              </Btn>
            </div>
          </Card>
        ) : null}

        {tokens.isError ? (
          <ApiFailureCard
            title="Personal tokens didn't load"
            error={tokens.error}
            requestLine="GET /me/tokens"
            onRetry={() => tokens.refetch()}
          />
        ) : isEmpty ? (
          <EmptyState
            compact
            icon="s-key"
            title="No personal tokens yet"
            meaning={
              <>
                tokens act as you — they carry your roles, scoped and expiring, and die with your
                membership; the secret is shown once
              </>
            }
            cta={
              <Btn variant="p" onClick={() => setCreateOpen(true)}>
                Create token (U7)
              </Btn>
            }
            cli="steloit token create"
          />
        ) : (
          <div className="tblwrap">
            <table className="tbl">
              <thead>
                <tr>
                  <th>Token</th>
                  <th>Prefix</th>
                  <th>Scope</th>
                  <th>Created</th>
                  <th>Last used</th>
                  <th>Expires</th>
                  <th aria-label="Actions" />
                </tr>
              </thead>
              <tbody>
                {tokens.isPending ? (
                  <SkeletonRows cols={7} />
                ) : (
                  (tokens.data ?? []).map((t) => {
                    const d = DISPLAY[t.name];
                    return (
                      <tr key={t.id}>
                        <td className="mono text-11p5">{t.name}</td>
                        <td className="mono text-11 text-ink3">{t.prefix}</td>
                        <td>{d?.scope ?? <Pill tone="mut">{t.scope}</Pill>}</td>
                        <td className="text-ink2">{d?.created ?? "—"}</td>
                        <td className="text-ink2">{d?.lastUsed ?? "—"}</td>
                        <td className="text-ink2">{d?.expires ?? "—"}</td>
                        <td className="text-right">
                          <Btn
                            variant="dgr"
                            onClick={() => setRevoking({ id: t.id, name: t.name })}
                          >
                            Revoke…
                          </Btn>
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        )}

        <p className="max-w-[640px] text-11 leading-relaxed text-ink3">
          Shown once, hash stored — same contract as org API keys (G8), different identity: these
          are you; those are the org's robots. If Acme removes you, every personal token dies with
          the membership.
        </p>
      </div>
      {reveal ? (
        <TokenReveal
          sub={`${reveal.name} · full — your roles · expires in 90 d`}
          token={reveal.created.token}
          onDone={() => setReveal(null)}
        />
      ) : null}
      {revoking ? (
        <ConfirmModal
          title={`Revoke ${revoking.name}`}
          consequence="Anything using it starts failing on the next request — CI jobs included."
          verb="Revoke token"
          pending={revokeToken.isPending}
          onConfirm={() => onRevoke(revoking.id, revoking.name)}
          onClose={() => setRevoking(null)}
        />
      ) : null}
    </main>
  );
}

export const Route = createFileRoute("/_app/account/tokens")({
  component: TokensPage,
});
