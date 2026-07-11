import { createFileRoute } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Flabel, Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
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
  const [armedId, setArmedId] = useState<string | null>(null);

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

  const onRevoke = (id: string, name: string) => {
    if (armedId !== id) {
      setArmedId(id);
      return;
    }
    revokeToken.mutate(
      { path: { tok: id } },
      {
        onSuccess: () => {
          setArmedId(null);
          toast.success(`Token ${name} revoked — it stops working immediately`);
          queryClient.invalidateQueries({ queryKey: listPersonalTokensQueryKey() });
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Personal tokens"
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
              {(tokens.data ?? []).map((t) => {
                const d = DISPLAY[t.name];
                const armed = armedId === t.id;
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
                        onClick={() => onRevoke(t.id, t.name)}
                        disabled={revokeToken.isPending}
                        disabledReason="Revoking…"
                      >
                        {armed ? "Confirm revoke" : "Revoke"}
                      </Btn>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>

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
    </main>
  );
}

export const Route = createFileRoute("/_app/account/tokens")({
  component: TokensPage,
});
