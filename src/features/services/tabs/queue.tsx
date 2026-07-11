import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Flabel, Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import type { Service } from "@/lib/api";

/**
 * D18 · Queue Settings — the delivery contract is immutable after create
 * (consumers were built against it); danger actions are honestly blocked by
 * what depends on the queue.
 */

export interface TabProps {
  svc: Service;
  org: string;
  project: string;
  env: string;
}

/** Inert display toggle — the DLQ state is read from the world, not editable here. */
function ToggleRow({ label, title }: { label: string; title?: string }) {
  return (
    <div className="flex items-center gap-3" title={title}>
      <span aria-hidden="true" className="relative h-[18px] w-8 shrink-0 rounded-full bg-steel">
        <span className="absolute top-[2px] left-[16px] h-[14px] w-[14px] rounded-full bg-white" />
      </span>
      <span className="text-12 text-ink2">
        {label}
        <span className="sr-only"> — on</span>
      </span>
    </div>
  );
}

export function QueueSettingsTab({ svc, env }: TabProps) {
  return (
    <>
      <Pghead
        before={<Glyph id="s-queue" />}
        title="Settings"
        sub={
          <span className="mono">
            {svc.name} · {env}
          </span>
        }
      />

      <Card className="flex flex-col gap-2.5 p-4">
        <Eyebrow>Delivery contract</Eyebrow>
        <div className="flex items-center gap-2">
          <Pill tone="st">at-least-once</Pill>
          <Pill tone="mut">immutable after create</Pill>
        </div>
        <p className="text-11p5 leading-relaxed text-ink2">
          consumers were built against this guarantee — changing it silently would break them;
          create a new queue and migrate deliberately instead
        </p>
      </Card>

      <Card className="flex flex-col gap-4 p-4">
        <div className="grid grid-cols-3 gap-4">
          <div>
            <Flabel htmlFor="queue-retries">Retries</Flabel>
            <Inp id="queue-retries" className="mono" value="5 × exponential backoff" readOnly />
          </div>
          <div>
            <Flabel htmlFor="queue-retention">Retention</Flabel>
            <Inp id="queue-retention" className="mono" value="7 days" readOnly />
          </div>
          <div>
            <Flabel htmlFor="queue-ttl">Message TTL</Flabel>
            <Inp id="queue-ttl" className="mono" value="none" readOnly />
          </div>
        </div>
        <ToggleRow label="Dead-letter queue · after 5 failures · alert when > 0 (pre-set)" />
      </Card>

      <Card className="flex flex-col gap-3 border-err/40 p-4">
        <Eyebrow className="text-err">Danger zone</Eyebrow>
        <div className="flex items-start gap-3">
          <Btn variant="dgr" disabled disabledReason="No purge endpoint in the spec (finding)">
            Purge 12 ready messages…
          </Btn>
          <p className="text-11 leading-relaxed text-ink3">
            type the queue name · in-flight messages complete · purge is an audit event with a
            required reason
          </p>
        </div>
        <div className="flex items-center gap-3 border-hair border-t pt-3">
          <Btn
            variant="dgr"
            disabled
            disabledReason="Blocked — 3 bindings and 1 schedule depend on this queue; detach them first"
          >
            Delete jobs…
          </Btn>
          <Pill tone="err">blocked — 3 bindings and 1 schedule depend on this queue</Pill>
        </div>
      </Card>
    </>
  );
}
