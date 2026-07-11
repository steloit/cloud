import { Link } from "@tanstack/react-router";
import { Icon } from "@/design-system/icon";
import { Nfoot, NitLink, Snav } from "./snav";

/**
 * Snav variant for the Assistant workspace (AI2/AI10/AI11): Ask, Insights,
 * Activity, Capabilities over one org-scoped conversation context.
 *
 * Route scope — 02-ia lists /assistant/* as top-level, but every AI frame
 * (AI2, AI10, AI11) scopes the workspace to org/project ("ecommerce ·
 * production"). Org-scoped (/$org/assistant/*) chosen so the shell chrome
 * (ctx bar + rail) keeps working — surfaced as a finding, not resolved
 * silently.
 */

export type AssistantLens = "ask" | "insights" | "activity" | "capabilities";

export function SnavAssistant({ org, active }: { org: string; active: AssistantLens }) {
  const params = { org };

  return (
    <Snav>
      <div className="snhead">
        <span className="glyph" style={{ background: "var(--assist-tint)" }}>
          <Icon id="s-ai" className="text-assist" />
        </span>
        <div>
          <div className="t">Assistant</div>
          <div className="u">ecommerce · production</div>
        </div>
      </div>
      <NitLink
        to="/$org/assistant/ask"
        params={params}
        icon="s-ai"
        label="Ask"
        on={active === "ask"}
      />
      <NitLink
        to="/$org/assistant/insights"
        params={params}
        icon="s-chart"
        label="Insights"
        count="3"
        on={active === "insights"}
      />
      <NitLink
        to="/$org/assistant/activity"
        params={params}
        icon="s-doc"
        label="Activity"
        on={active === "activity"}
      />
      {/* Capabilities (AI12) — the route file is owned outside this wave; the
          link is spec'd by the frames and lands when that leaf exists. */}
      <NitLink
        to="/$org/assistant/capabilities"
        params={params}
        icon="s-grid"
        label="Capabilities"
        on={active === "capabilities"}
      />
      <Nfoot>
        <div className="px-1.5">Suggests, never acts · explainable · within your permissions.</div>
        <div className="px-1.5 pt-1">
          <Link
            to="/$org/settings/policies/ai-assistant"
            params={params}
            className="text-steel hover:underline"
          >
            Disable in Policies (AI3)
          </Link>
        </div>
      </Nfoot>
    </Snav>
  );
}
