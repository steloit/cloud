import { createFileRoute, Link } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Icon, type IconId } from "@/design-system/icon";

/**
 * AI12 · Capabilities — a directory of what the AI does and where, with the
 * safety boundary and the org-wide off-switch (AI3).
 */

interface Capability {
  icon: IconId;
  title: string;
  desc: string;
  where: string;
  action: { label: string; to?: string; search?: Record<string, unknown> };
}

const CAPABILITIES: Capability[] = [
  {
    icon: "s-ai",
    title: "Describe to provision",
    desc: "Describe an app in plain language; get a reviewable stack with per-service reasoning and a live estimate.",
    where: "Appears in: the Create surface (AI1)",
    action: { label: "Try it", to: "/$org/create", search: { env: "production" } },
  },
  {
    icon: "s-db",
    title: "Database insights",
    desc: "Slow-query analysis, index recommendations, query explanation and migration drafts — as reviewable proposals.",
    where: "Appears in: PostgreSQL insights (W10)",
    action: {
      label: "Open",
      to: "/$org/$project/svc/$service/insights",
      search: { env: "production" },
    },
  },
  {
    icon: "s-chip",
    title: "Cache optimization",
    desc: "Memory pressure, eviction policy and TTL strategy for Valkey, applied through Settings.",
    where: "Appears in: Valkey insights (AI6)",
    action: { label: "Open", to: "/$org/$project/svc/$service/ai", search: { env: "production" } },
  },
  {
    icon: "s-bucket",
    title: "Storage lifecycle & cost",
    desc: "Object age/access analysis with lifecycle rules and cost savings — dry-run first, no deletes.",
    where: "Appears in: Object Storage (AI7)",
    action: { label: "Open", to: "/$org/$project/svc/$service/ai", search: { env: "production" } },
  },
  {
    icon: "s-queue",
    title: "Queue & DLQ analysis",
    desc: "Correlates dead letters to their cause, tells a code fix from a retry, drafts the fix.",
    where: "Appears in: Queue insights (AI8)",
    action: { label: "Open", to: "/$org/$project/svc/$service/ai", search: { env: "production" } },
  },
  {
    icon: "s-cron",
    title: "Incident copiloting",
    desc: "Combines metrics, logs, traces and events to explain what happened and propose a fix.",
    where: "Appears in: Observe · Events (O4)",
    action: {
      label: "Open",
      to: "/$org/$project/observe/events",
      search: { env: "production" },
    },
  },
];

/** Canon service per capability card (the deep links land on the incident's services). */
const CARD_SERVICE: Record<string, string> = {
  "Database insights": "db-main",
  "Cache optimization": "cache",
  "Storage lifecycle & cost": "assets",
  "Queue & DLQ analysis": "jobs",
};

function CapabilitiesPage() {
  const { org } = Route.useParams();

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Capabilities"
          sub="Everything the Assistant can do and where each surfaces — all under the four laws, none acting without you."
        />
        <div className="grid grid-cols-2 gap-3.5">
          {CAPABILITIES.map((cap) => (
            <Card key={cap.title} className="flex flex-col gap-2 p-4">
              <div className="flex items-center gap-2.5">
                <span className="glyph">
                  <Icon id={cap.icon} />
                </span>
                <b className="text-[13px]">{cap.title}</b>
              </div>
              <p className="text-[12px] leading-relaxed text-ink2">{cap.desc}</p>
              <div className="mt-auto flex items-center gap-2.5">
                <span className="text-[10.5px] text-ink3">{cap.where}</span>
                <span className="ml-auto">
                  {cap.action.to ? (
                    <Link
                      to={cap.action.to}
                      params={{
                        org,
                        project: "ecommerce",
                        service: CARD_SERVICE[cap.title] ?? "db-main",
                      }}
                      search={cap.action.search}
                    >
                      <Btn variant="s" className="h-6 px-2.5 text-[10.5px]">
                        {cap.action.label}
                      </Btn>
                    </Link>
                  ) : null}
                </span>
              </div>
            </Card>
          ))}
        </div>
        <Card className="flex items-center gap-3 p-3.5">
          <Icon id="s-shield" className="h-4 w-4 shrink-0 text-ink2" />
          <p className="text-[11.5px] leading-relaxed text-ink2">
            The Assistant never performs IAM, secrets, network or destructive actions directly — it
            explains or drafts, and you execute through the standard workflow. Every capability can
            be turned off org-wide in{" "}
            <Link
              to="/$org/settings/policies/ai-assistant"
              params={{ org }}
              className="font-medium text-steel"
            >
              Policies (AI3)
            </Link>
            ; the platform works fully without any of it.
          </p>
        </Card>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/assistant/capabilities")({
  component: CapabilitiesPage,
});
