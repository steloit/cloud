import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Copybit } from "@/design-system/copybit";
import { EmptyRow } from "@/design-system/empty-state";
import { Icon } from "@/design-system/icon";
import { Pill } from "@/design-system/pill";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { useAuditEvents, useOrgs } from "@/features/org/hooks";
import type { Event } from "@/lib/api";

/** W12 · Org audit log — who decided, in plain language; humans, tokens, and "via assistant". */

function actorCell(e: Event) {
  const detail = (e.detail ?? {}) as Record<string, unknown>;
  const note = typeof detail.actor_note === "string" ? detail.actor_note : undefined;
  if (note) {
    // System/token rows show only the pill; human-with-note rows show both.
    const isHuman = e.via === "user";
    return (
      <span className="flex items-center gap-2">
        {isHuman ? <b>{e.actor}</b> : null}
        <Pill tone="mut">{note}</Pill>
      </span>
    );
  }
  if (e.via === "assistant") {
    return (
      <span className="flex items-center gap-2">
        <b>{e.actor}</b>
        <Pill tone="ai">via assistant</Pill>
      </span>
    );
  }
  return <b>{e.actor}</b>;
}

function timeCell(iso: string): string {
  return new Intl.DateTimeFormat("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    timeZone: "Asia/Kolkata",
  }).format(new Date(iso));
}

function exportCsv(events: Event[]) {
  const rows = [
    ["time", "actor", "via", "action", "target", "event_id"],
    ...events.map((e) => [
      e.at,
      e.actor,
      e.via ?? "",
      e.action ?? "",
      String((e.detail as Record<string, unknown> | undefined)?.target ?? ""),
      e.id,
    ]),
  ];
  const csv = rows.map((r) => r.map((c) => `"${c.replaceAll('"', '""')}"`).join(",")).join("\n");
  const url = URL.createObjectURL(new Blob([csv], { type: "text/csv" }));
  const a = document.createElement("a");
  a.href = url;
  a.download = "steloit-audit.csv";
  a.click();
  URL.revokeObjectURL(url);
}

function AuditLogPage() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const events = useAuditEvents(org);

  // The filter row below is frame-fixed W12 chrome (static pickers + an
  // active event-type chip), so an empty result here reads as filtered-out,
  // not as an empty history. Making the chip actually filter would drop the
  // canon lifecycle/alert rows the frame shows alongside it — finding — so
  // the flag is a constant and the no-filter branch is canon-unreachable,
  // wired anyway (same status as other canon-unreachable empties).
  const filterActive: boolean = true;

  return (
    <>
      <SnavSettings org={org} orgName={orgRecord?.name ?? org} project="ecommerce" active="audit" />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Audit log"
            sub="Immutable, from day one · every state change by human, machine, or assistant"
          >
            <Btn variant="s" onClick={() => exportCsv(events.data ?? [])}>
              Export
            </Btn>
          </Pghead>

          <div className="flex items-center gap-2">
            <span className="envpill">
              Last 24 hours <Icon id="s-chevd" className="h-[11px] w-[11px]" />
            </span>
            <span className="envpill">
              All projects <Icon id="s-chevd" className="h-[11px] w-[11px]" />
            </span>
            <span className="envpill">
              All actors <Icon id="s-chevd" className="h-[11px] w-[11px]" />
            </span>
            <span className="envpill text-steel">event: config, policy, deploy ×</span>
            <span className="ml-auto">
              <Copybit>steloit audit list --since 24h</Copybit>
            </span>
          </div>

          {events.isError ? (
            <ApiFailureCard
              title="Audit log didn't load"
              error={events.error}
              requestLine={`GET /orgs/${org}/audit`}
              onRetry={() => events.refetch()}
            />
          ) : (
            <div className="tblwrap">
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Time</th>
                    <th>Actor</th>
                    <th>Action</th>
                    <th>Target</th>
                    <th>Event ID</th>
                  </tr>
                </thead>
                <tbody>
                  {events.isPending ? (
                    [0, 1, 2, 3].map((i) => (
                      <tr key={i}>
                        <td colSpan={5}>
                          <div className="h-4 animate-pulse rounded bg-surface2" />
                        </td>
                      </tr>
                    ))
                  ) : (events.data ?? []).length === 0 ? (
                    <EmptyRow cols={5}>
                      {filterActive
                        ? "No events match this filter — widen the time range or clear the event filter"
                        : "No audit events yet — every state change lands here from day one"}
                    </EmptyRow>
                  ) : (
                    (events.data ?? []).map((e) => {
                      const detail = (e.detail ?? {}) as Record<string, unknown>;
                      return (
                        <tr key={e.id}>
                          <td className="mono text-ink3">{timeCell(e.at)}</td>
                          <td>{actorCell(e)}</td>
                          <td>
                            <span className="flex items-center gap-2">
                              {e.action}
                              {detail.denied_by === "policy" ? (
                                <Pill tone="err">policy</Pill>
                              ) : null}
                            </span>
                          </td>
                          <td className="mono text-ink2">{String(detail.target ?? "")}</td>
                          <td className="mono text-ink3">{e.id}</td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>
          )}

          <p className="text-11 text-ink3">
            Audit events are append-only and retained per org policy · assistant-applied changes
            always carry the approving human's identity.
          </p>
        </div>
      </main>
    </>
  );
}

export const Route = createFileRoute("/_app/$org/settings/audit")({
  component: AuditLogPage,
});
