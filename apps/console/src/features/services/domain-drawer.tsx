import { useMutation } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Icon } from "@/design-system/icon";
import { Flabel } from "@/design-system/inp";
import { Drawer } from "@/design-system/overlay";
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
    <Drawer
      title="Add custom domain"
      sub={
        <>
          {svc.name} · {project} / {env}
        </>
      }
      onClose={onClose}
      footer={
        <>
          <span className="text-10p5 text-ink3">No cost — domains &amp; certs are included</span>
          <span className="flex-1" />
          <Btn variant="s" onClick={onClose}>
            Close — keep checking
          </Btn>
          <Btn
            variant="s"
            disabled
            disabledReason="No recheck endpoint in the spec — the 60 s poll is the only path (finding)"
          >
            Recheck now
          </Btn>
        </>
      }
    >
      <div className="flex items-center gap-2 text-11">
        <span className="flex items-center gap-1 font-semibold text-ok">
          <Icon id="s-check" className="h-[11px] w-[11px]" /> Domain added
        </span>
        <span className="text-ink3">—</span>
        <span className="font-semibold text-warn">② Verifying DNS</span>
        <span className="text-ink3">—</span>
        <span className="text-ink3">③ Cert issued</span>
      </div>

      <div>
        <Flabel>Domain</Flabel>
        <div className="flex items-center gap-2.5">
          <span className="mono text-12p5">{domain}</span>
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
        <Flabel aside={<span className="mono text-10p5 text-ink3">every 60 s</span>}>
          Live check
        </Flabel>
        <div className="logwell">
          <div className="lv-w whitespace-pre">dig CNAME {domain} → not found yet</div>
          <div className="whitespace-pre text-ok">
            dig TXT {txt?.name ?? `_steloit.${domain}`} → ✓ {txt?.value ?? "…"}
          </div>
        </div>
      </div>

      <Card className="p-3.5 text-11 leading-relaxed text-ink2">
        You can close this. Checking continues in the background; the certificate issues and
        attaches automatically the moment DNS resolves (usually minutes, up to 48 h). You'll get a
        bell when it's live — and TLS is never optional (never gated, either).
      </Card>
    </Drawer>
  );
}
