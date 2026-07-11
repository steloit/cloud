import { useMutation } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { PRODUCT_ICON } from "@/app/shell/rail";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Icon } from "@/design-system/icon";
import { Flabel, Inp } from "@/design-system/inp";
import { captureTemplateMutation, errorMessage } from "@/lib/api";
import { fmtMoney } from "@/lib/fmt";
import { cn } from "@/lib/utils";

/**
 * T3 · Save-as-template — the capture flow (from W2's button and T1's ＋):
 * pick services, placeholder bindings, the secrets-never-captured contract,
 * estimate from birth. Un-nested route (`templates_.`) because T1 renders no
 * Outlet — same URL, flat file.
 */

const SERVICES = [
  {
    name: "api",
    product: "web" as const,
    desc: "Web · autoscale 2–6",
    cents: 6100,
    included: true,
  },
  {
    name: "worker",
    product: "worker" as const,
    desc: "background jobs",
    cents: 2200,
    included: true,
  },
  {
    name: "jobs",
    product: "queue" as const,
    desc: "Queue · DLQ + schedules",
    cents: 1200,
    included: true,
  },
  { name: "cache", product: "valkey" as const, desc: "Valkey 2 GB", cents: 2200, included: true },
  {
    name: "db-main",
    product: "postgres" as const,
    desc: "PostgreSQL — excluded",
    cents: 5800,
    included: false,
  },
  {
    name: "assets",
    product: "storage" as const,
    desc: "Object Storage — excluded",
    cents: 900,
    included: false,
  },
];

function SaveTemplatePage() {
  const { org } = Route.useParams();
  const navigate = useNavigate();
  const capture = useMutation(captureTemplateMutation());
  const [name, setName] = useState("checkout-stack");
  const [included, setIncluded] = useState<Record<string, boolean>>(
    Object.fromEntries(SERVICES.map((s) => [s.name, s.included])),
  );

  const selected = SERVICES.filter((s) => included[s.name]);
  const totalCents = selected.reduce((sum, s) => sum + s.cents, 0);

  const save = () =>
    capture.mutate(
      {
        path: { org },
        body: {
          name,
          visibility: "org",
          source: { project: "ecommerce", env: "production" },
          services: selected.map((s) => s.name),
        },
      },
      {
        onSuccess: () => {
          toast.success(`${name} saved — nothing provisions, nothing bills`);
          navigate({ to: "/$org/settings/templates", params: { org } });
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Save as template"
          sub={
            <>
              from <span className="mono">ecommerce / production</span> — pick what's included; the
              shape is copied, secrets never are.
            </>
          }
        />
        <div className="flex gap-4">
          <div className="flex max-w-[760px] flex-1 flex-col gap-3.5">
            <Card className="flex flex-col gap-3 p-4">
              <div>
                <Flabel htmlFor="tpl-name">Template name</Flabel>
                <Inp
                  id="tpl-name"
                  className="mono"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </div>
              <div>
                <Flabel>Visibility</Flabel>
                <div className="inp w-[220px]">Organization ▾</div>
              </div>
            </Card>
            <Card className="flex flex-col p-4">
              <div className="mb-2 flex items-baseline gap-2.5">
                <Eyebrow>Include services</Eyebrow>
                <span className="text-[10.5px] text-ink3">
                  {selected.length} of {SERVICES.length} selected
                </span>
              </div>
              {SERVICES.map((svc) => (
                <label
                  key={svc.name}
                  className={cn(
                    "flex cursor-pointer items-center gap-3 border-hair border-b py-2.5 last:border-b-0",
                    !included[svc.name] && "opacity-55",
                  )}
                >
                  <input
                    type="checkbox"
                    checked={Boolean(included[svc.name])}
                    onChange={() =>
                      setIncluded((prev) => ({ ...prev, [svc.name]: !prev[svc.name] }))
                    }
                  />
                  <Icon id={PRODUCT_ICON[svc.product]} className="h-3.5 w-3.5 text-ink3" />
                  <b className="text-[12.5px]">{svc.name}</b>
                  <span className="text-[11px] text-ink3">{svc.desc}</span>
                  <span className="mono ml-auto text-[11.5px]">{fmtMoney(svc.cents)}</span>
                </label>
              ))}
              <p className="mt-2.5 text-[11px] leading-relaxed text-ink3">
                Bindings between selected services are kept as placeholders; a binding to an
                excluded service (api → db-main) becomes a <b>required input</b> the consumer wires
                at create.
              </p>
            </Card>
            <Card className="flex items-start gap-3 p-4">
              <Icon id="s-key" className="mt-0.5 h-4 w-4 shrink-0 text-ink2" />
              <p className="text-[12px] leading-relaxed text-ink2">
                <b>Secrets, data and keys are never captured.</b> Every consumer gets fresh
                credentials, minted at create — the template is a recipe, not a pantry.
              </p>
            </Card>
          </div>
          <div className="flex w-[320px] shrink-0 flex-col gap-3">
            <Card className="flex flex-col gap-2 p-4">
              <Eyebrow>Instantiation estimate</Eyebrow>
              <div className="mono text-[26px] font-semibold tracking-[-0.5px]">
                {fmtMoney(totalCents)}
                <span className="ml-1 text-[11px] font-normal text-ink3">/mo</span>
              </div>
              <p className="text-[10.5px] leading-relaxed text-ink3">
                What creating from this template costs at today's prices — shown here so the
                template carries its price from birth, like every template in the gallery (A8).
              </p>
              <Btn
                variant="p"
                className="mt-1 justify-center"
                onClick={save}
                disabled={capture.isPending || !name.trim() || selected.length === 0}
              >
                Save {name || "template"}
              </Btn>
              <Btn
                variant="s"
                className="justify-center"
                onClick={() => navigate({ to: "/$org/settings/templates", params: { org } })}
              >
                Cancel
              </Btn>
              <Copybit>{`steloit template save ${name || "checkout-stack"} --from ecommerce/production --services ${selected.map((s) => s.name).join(",")}`}</Copybit>
              <p className="text-[10.5px] leading-relaxed text-ink3">
                Saving creates the asset only — nothing provisions, nothing bills. Manage it under
                Settings → Templates (T1).
              </p>
            </Card>
          </div>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/settings/templates_/new")({
  component: SaveTemplatePage,
});
