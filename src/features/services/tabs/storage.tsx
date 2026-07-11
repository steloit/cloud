import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Pill } from "@/design-system/pill";
import type { Service } from "@/lib/api";

/**
 * D16 · Object Storage Settings — access is a policy question (public is a
 * request routed to an Admin, never a toggle); delete is honestly blocked by
 * what depends on the bucket.
 */

export interface TabProps {
  svc: Service;
  org: string;
  project: string;
  env: string;
}

/** Inert display toggle — the versioning state is read from the world, not editable here. */
function ToggleRow({ label, title }: { label: string; title?: string }) {
  return (
    <div className="flex items-center gap-3" title={title}>
      <span aria-hidden="true" className="relative h-[18px] w-8 shrink-0 rounded-full bg-steel">
        <span className="absolute top-[2px] left-[16px] h-[14px] w-[14px] rounded-full bg-white" />
      </span>
      <span className="text-[12px] text-ink2">
        {label}
        <span className="sr-only"> — on</span>
      </span>
    </div>
  );
}

export function StorageSettingsTab({ svc, env }: TabProps) {
  return (
    <>
      <Pghead
        before={<Glyph id="s-bucket" />}
        title="Settings"
        sub={
          <span className="mono">
            {svc.name} · {env}
          </span>
        }
      />

      <Card className="flex flex-col gap-3 p-4">
        <Eyebrow>Access</Eyebrow>
        <div className="flex items-center gap-2.5">
          <Pill tone="mut">private · signed URLs</Pill>
          <span className="flex-1" />
          <Btn
            variant="s"
            disabled
            disabledReason="No access-request endpoint in the spec (finding)"
          >
            Request public CDN…
          </Btn>
        </div>
        <p className="text-[11.5px] leading-relaxed text-ink2">
          routes to an Admin — org policy <span className="mono">public-buckets</span> requires
          named approval; the grant, if made, is an audit event
        </p>
        <div className="flex flex-col gap-2.5 border-hair border-t pt-3">
          <div className="flex items-center justify-between text-[12px]">
            <span className="text-ink2">Signed URL default expiry</span>
            <span className="mono">15 min</span>
          </div>
          <div className="flex items-center justify-between text-[12px]">
            <span className="text-ink2">CORS origins</span>
            <span className="flex items-center gap-2">
              <span className="mono">acme-store.com</span>
              <Pill tone="mut">+1</Pill>
            </span>
          </div>
          <ToggleRow label="Versioning · soft delete 30 d" />
        </div>
      </Card>

      <Card className="flex flex-col gap-3 border-err/40 p-4">
        <Eyebrow className="text-err">Danger zone</Eyebrow>
        <div className="flex items-center gap-3">
          <Btn
            variant="dgr"
            disabled
            disabledReason="Blocked — 1.21 M objects and 2 bindings; empty and detach first"
          >
            Delete assets…
          </Btn>
          <Pill tone="err">blocked — 1.21 M objects and 2 bindings</Pill>
        </div>
        <p className="text-[11px] leading-relaxed text-ink3">
          empty the bucket (or let lifecycle drain it), detach bindings, then type the name — there
          is no force-delete on 48 GB of customer images
        </p>
      </Card>
    </>
  );
}
