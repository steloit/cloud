import { useState } from "react";
import { Pghead } from "@/app/shell/pghead";
import { PRODUCT_LABEL } from "@/app/shell/rail";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Flabel, Inp } from "@/design-system/inp";
import { Drawer } from "@/design-system/overlay";
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
  const [editingVars, setEditingVars] = useState(false);
  return (
    <>
      {/* Finding: frame D23 shows the bare "Settings" title — the design-system
          "Area · Thing" h1 grammar wins per the audit's P1 ruling. */}
      <Pghead
        title={`${PRODUCT_LABEL[svc.product]} · Settings`}
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
              </tr>
            </thead>
            <tbody>
              {ENV_VARS.map((v) => (
                <tr key={v.key}>
                  <td className="mono">{v.key}</td>
                  <td className="mono">{v.value}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div>
          <Btn variant="s" onClick={() => setEditingVars(true)}>
            Edit variables…
          </Btn>
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

      {editingVars ? (
        <EditVariablesDrawer svc={svc} env={env} onClose={() => setEditingVars(false)} />
      ) : null}
    </>
  );
}

/**
 * Edit variables — the 424 add-secret sheet. Apply stays gated with the
 * file's finding: env-var CRUD has no endpoint of its own yet.
 */
function EditVariablesDrawer({
  svc,
  env,
  onClose,
}: {
  svc: Service;
  env: string;
  onClose: () => void;
}) {
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  return (
    <Drawer
      title="Edit variables"
      sub={
        <>
          {svc.name} · {env}
        </>
      }
      onClose={onClose}
      footer={
        <>
          <span className="flex-1" />
          <Btn variant="s" onClick={onClose}>
            Cancel
          </Btn>
          <Btn
            variant="p"
            disabled
            disabledReason="Env-var apply lands with a vars field on updateService (finding)"
          >
            Apply
          </Btn>
        </>
      }
    >
      <div className="flex flex-col gap-1.5">
        <div className="eyebrow">Current variables</div>
        {ENV_VARS.map((v) => (
          <div
            key={v.key}
            className="flex items-center gap-2.5 rounded-lg border border-hair px-3 py-2"
          >
            <span className="mono text-11p5">{v.key}</span>
            <span className="flex-1" />
            <span className="mono text-11 text-ink3">••••••••</span>
          </div>
        ))}
      </div>

      <div className="flex flex-col gap-2.5">
        <div className="eyebrow">Add variable</div>
        <div>
          <Flabel htmlFor="envvar-key">Key</Flabel>
          <Inp
            id="envvar-key"
            className="mono"
            placeholder="KEY"
            value={key}
            onChange={(e) => setKey(e.target.value)}
          />
        </div>
        <div>
          <Flabel htmlFor="envvar-value">Value</Flabel>
          <Inp
            id="envvar-value"
            className="mono"
            value={value}
            onChange={(e) => setValue(e.target.value)}
          />
        </div>
        <p className="text-10p5 text-ink3">
          secrets are write-only after save — read access is the binding's job
        </p>
      </div>
    </Drawer>
  );
}
