import { useMutation } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Flabel } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { addDomainMutation, type Service } from "@/lib/api";

/**
 * U5 · Add custom domain drawer (424px, over D21). POST fires once on open
 * with the prefilled domain; everything below the stepper renders from the
 * 201 response (status `verifying` + the CNAME/TXT records to add).
 */

interface AddDomainDrawerProps {
  svc: Service;
  project: string;
  env: string;
  /** Prefilled from the D21 add-domain input. */
  domain: string;
  onClose: () => void;
}

export function AddDomainDrawer({ svc, project, env, domain, onClose }: AddDomainDrawerProps) {
  const add = useMutation(addDomainMutation());
  const fired = useRef(false);

  const { mutate } = add;
  useEffect(() => {
    // Fire once on open (ref guards StrictMode's double effect run).
    if (fired.current) return;
    fired.current = true;
    mutate({ path: { service: svc.id }, body: { domain } });
  }, [mutate, svc.id, domain]);

  const records = add.data?.records ?? [];
  const txt = records.find((r) => r.type === "TXT");

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: scrim dismiss mirrors Esc
    // biome-ignore lint/a11y/useKeyWithClickEvents: the ✕ button is the keyboard path
    <div className="fixed inset-0 z-40 bg-[rgba(6,9,12,0.44)]" onClick={onClose}>
      {/* biome-ignore lint/a11y/useKeyWithClickEvents: click only stops scrim dismiss */}
      <div
        className="absolute top-0 right-0 flex h-full w-[424px] flex-col border-hair border-l bg-surface shadow-e2"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label="Add custom domain"
      >
        <div className="flex items-start gap-3 border-hair border-b px-5 py-4">
          <div>
            <div className="text-[14px] font-semibold">Add custom domain</div>
            <div className="mono mt-0.5 text-[10.5px] text-ink3">
              {svc.name} · {project} / {env}
            </div>
          </div>
          <button
            type="button"
            className="ml-auto text-[13px] text-ink3 hover:text-ink1"
            onClick={onClose}
            aria-label="Close"
          >
            ✕
          </button>
        </div>

        <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">
          <div className="flex items-center gap-2 text-[11px]">
            <span className="font-semibold text-ok">✓ Domain added</span>
            <span className="text-ink3">—</span>
            <span className="font-semibold text-warn">② Verifying DNS</span>
            <span className="text-ink3">—</span>
            <span className="text-ink3">③ Cert issued</span>
          </div>

          <div>
            <Flabel>Domain</Flabel>
            <div className="flex items-center gap-2.5">
              <span className="mono text-[12.5px]">{domain}</span>
              <Pill tone="warn">verifying</Pill>
            </div>
          </div>

          <div>
            <Flabel>Add these records at your DNS provider</Flabel>
            <div className="logwell">
              {records.length > 0 ? (
                records.map((r) => (
                  <div key={`${r.type}-${r.name}`} className="whitespace-pre">
                    {(r.type ?? "").padEnd(6)} {r.name} → {r.value}
                  </div>
                ))
              ) : (
                <div className="t">requesting records…</div>
              )}
            </div>
          </div>

          <div>
            <Flabel aside={<span className="mono text-[10.5px] text-ink3">every 60 s</span>}>
              Live check
            </Flabel>
            <div className="logwell">
              <div className="lv-w whitespace-pre">dig CNAME {domain} → not found yet</div>
              <div className="whitespace-pre text-ok">
                dig TXT {txt?.name ?? `_steloit.${domain}`} → ✓ {txt?.value ?? "…"}
              </div>
            </div>
          </div>

          <Card className="p-3.5 text-[11px] leading-relaxed text-ink2">
            You can close this. Checking continues in the background; the certificate issues and
            attaches automatically the moment DNS resolves (usually minutes, up to 48 h). You'll get
            a bell when it's live — and TLS is never optional (never gated, either).
          </Card>
        </div>

        <div className="flex items-center gap-2 border-hair border-t px-5 py-3.5">
          <span className="text-[10.5px] text-ink3">
            No cost — domains &amp; certs are included
          </span>
          <span className="flex-1" />
          <Btn variant="s" onClick={onClose}>
            Close — keep checking
          </Btn>
          <Btn variant="s" className="opacity-55">
            Recheck now
          </Btn>
        </div>
      </div>
    </div>
  );
}
