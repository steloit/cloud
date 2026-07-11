import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Pill } from "@/design-system/pill";
import type { Service } from "@/lib/api";

/**
 * D23 · Web service Settings tab. Carries its own Pghead so the settings
 * route stays a thin product switch; the worker product reuses this tab
 * (accepts any product). Values below (source repo, env vars, the 214 req/s
 * pill) are frame-fixed from the D23 gallery frame.
 */

interface WebSettingsTabProps {
  svc: Service;
  org: string;
  project: string;
  env: string;
}

const ENV_VARS = [
  { key: "CHECKOUT_FLAGS", value: "gift_cards=on" },
  { key: "LOG_LEVEL", value: "info" },
];

export function WebSettingsTab({ svc, env }: WebSettingsTabProps) {
  return (
    <>
      <Pghead
        title="Settings"
        sub={
          <span className="mono">
            {svc.name} · {env}
          </span>
        }
      />

      <Card className="flex flex-col gap-2.5 p-4">
        <Eyebrow>Build &amp; runtime</Eyebrow>
        <div className="flex items-center gap-2.5 text-12">
          <span className="w-28 text-ink3">Source</span>
          <span className="mono">acme/store · main</span>
          <Pill tone="mut">org Git</Pill>
        </div>
        <div className="flex items-center gap-2.5 text-12">
          <span className="w-28 text-ink3">Dockerfile</span>
          <span className="mono">./Dockerfile</span>
        </div>
        <div className="flex items-center gap-2.5 text-12">
          <span className="w-28 text-ink3">Health check</span>
          <span className="mono">GET /healthz · 5 s</span>
        </div>
      </Card>

      <Card className="flex flex-col gap-2.5 p-4">
        <div className="text-12 font-semibold">
          Custom environment variables — non-secret config only — secrets arrive via bindings, not
          here
        </div>
        <div className="tblwrap">
          <table className="tbl">
            <thead>
              <tr>
                <th>Key</th>
                <th>Value</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {ENV_VARS.map((v) => (
                <tr key={v.key}>
                  <td className="mono">{v.key}</td>
                  <td className="mono">{v.value}</td>
                  <td className="text-right">
                    <Btn
                      variant="gh"
                      disabled
                      disabledReason="Env-var CRUD folds into updateService.shape — apply flow lands in Phase 5"
                    >
                      Edit
                    </Btn>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="text-11 leading-relaxed text-ink3">
          Pasting something that looks like a secret triggers a block with a pointer to Bindings —
          the console refuses to become a .env file.
        </p>
      </Card>

      <Card className="flex flex-col gap-2.5 border-err p-4">
        <Eyebrow className="text-err">Danger zone</Eyebrow>
        <div className="flex items-center gap-2.5">
          <Btn
            variant="dgr"
            disabled
            disabledReason="serving live traffic — detach the domain first, then type-to-confirm"
          >
            Delete {svc.name}…
          </Btn>
          <Pill tone="warn">serving 214 req/s on api.acme-store.com</Pill>
        </div>
        <p className="text-11 leading-relaxed text-ink2">
          live-traffic deletes require the domain to be detached first, then type-to-confirm — 3
          outbound bindings are revoked with it
        </p>
      </Card>
    </>
  );
}
