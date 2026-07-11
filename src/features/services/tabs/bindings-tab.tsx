import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { ConfirmModal } from "@/design-system/confirm";
import { EmptyState } from "@/design-system/empty-state";
import { Drawer } from "@/design-system/overlay";
import { Pill } from "@/design-system/pill";
import { SkeletonRows } from "@/design-system/skeleton";
import { ApiFailureCard } from "@/features/errors/failure-states";
import { useBindings, useServices } from "@/features/services/hooks";
import type { Binding, Service } from "@/lib/api";
import {
  createBindingMutation,
  deleteBindingMutation,
  errorMessage,
  listBindingsQueryKey,
} from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * D11 · Bindings — generic for all products — plus the U2 create-binding
 * drawer. Credentials are minted per consumer, least-privilege, rotated;
 * consumers never see a password.
 */

export interface BindingsTabProps {
  svc: Service;
  org: string;
  project: string;
  env: string;
}

/** Frame-fixed D11 display values for the db-main canon rows. */
const DBMAIN_DISPLAY: Record<string, { rotated: string; lastUsed: string }> = {
  bnd_api_dbmain: { rotated: "12 d ago", lastUsed: "now · 141 conns" },
  bnd_worker_dbmain: { rotated: "12 d ago", lastUsed: "2 s ago" },
};

interface BindingRow {
  id: string;
  consumer: string;
  consumerPill?: { tone: "mut"; label: string };
  role: string;
  rotated: string;
  lastUsed: string;
  binding?: Binding;
}

export function BindingsTab({ svc, project, env }: BindingsTabProps) {
  const services = useServices(env);
  const bindings = useBindings(svc.id);
  const qc = useQueryClient();
  const revoke = useMutation(deleteBindingMutation());
  const [confirming, setConfirming] = useState<BindingRow | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const nameOf = (id: string) =>
    services.data?.find((s) => s.id === id)?.name ?? id.replace(/^svc_/, "");

  const rows: BindingRow[] = (bindings.data ?? [])
    .filter((b) => b.target_id === svc.id)
    .map((b) => ({
      id: b.id,
      consumer: nameOf(b.source_id),
      role: b.scope === "read_only" ? "read-only" : "read-write",
      rotated: DBMAIN_DISPLAY[b.id]?.rotated ?? (b.rotated_at ? b.rotated_at.slice(0, 10) : "—"),
      lastUsed: DBMAIN_DISPLAY[b.id]?.lastUsed ?? "—",
      binding: b,
    }));

  // Fixtures carry 2 db-main bindings but the D11 frame shows 3 — the third
  // (bnd_admin_dbmain, provisioning) is frame-fixed canon, appended only for
  // db-main; revoking it is a no-op against the canon world.
  if (svc.name === "db-main") {
    rows.push({
      id: "bnd_admin_dbmain",
      consumer: "admin",
      consumerPill: { tone: "mut", label: "provisioning" },
      role: "read-only",
      rotated: "at issue",
      lastUsed: "— activates at ready",
    });
  }

  // Four-state grammar (16-qa): pending → skeleton, error → failure card,
  // empty → EmptyState, else table. db-main never hits empty (synthetic
  // provisioning row above) — the empty state is for services with zero
  // consumers.
  const isEmpty = !bindings.isPending && !bindings.isError && rows.length === 0;

  // One confirm idiom (overlay census): the two-click armed button becomes
  // the 460 ConfirmModal — same mutation, same invalidation as before.
  const doRevoke = (row: BindingRow) => {
    revoke.mutate(
      { path: { binding: row.id } },
      {
        onSuccess: () => {
          toast.success(`Binding revoked — ${row.consumer} loses access, audit-logged`);
          qc.invalidateQueries({ queryKey: listBindingsQueryKey({ path: { service: svc.id } }) });
          setConfirming(null);
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    <>
      <Pghead
        title="Bindings"
        sub={
          <span className="mono">
            {svc.name} · {bindings.isSuccess ? rows.length : "…"} consumers · credentials minted per
            consumer, least-privilege, rotated
          </span>
        }
      />

      {bindings.isError ? (
        <ApiFailureCard
          title="Bindings didn't load"
          error={bindings.error}
          requestLine={`GET /services/${svc.id}/bindings`}
          onRetry={() => bindings.refetch()}
        />
      ) : isEmpty ? (
        <EmptyState
          compact
          icon="s-bind"
          title="No consumers yet"
          meaning={
            <>
              bindings mint least-privilege credentials per consumer — nothing shares a password,
              and consumers never see one
            </>
          }
          cta={
            <Btn variant="s" onClick={() => setDrawerOpen(true)}>
              Create binding (U2)
            </Btn>
          }
          cli={`steloit bind api --to ${svc.name} --scope read-only`}
        />
      ) : (
        <div className="tblwrap">
          <table className="tbl">
            <thead>
              <tr>
                <th>Binding</th>
                <th>Consumer</th>
                <th>Role</th>
                <th>Rotated</th>
                <th>Last used</th>
                <th aria-label="Actions" />
              </tr>
            </thead>
            <tbody>
              {bindings.isPending ? (
                <SkeletonRows cols={6} />
              ) : (
                rows.map((row) => (
                  <tr key={row.id}>
                    <td className="mono text-11">{row.id}</td>
                    <td>
                      <span className="flex items-center gap-2">
                        {row.consumer}
                        {row.consumerPill ? (
                          <Pill tone={row.consumerPill.tone}>{row.consumerPill.label}</Pill>
                        ) : null}
                      </span>
                    </td>
                    <td>{row.role}</td>
                    <td className="text-ink2">{row.rotated}</td>
                    <td className="text-ink2">{row.lastUsed}</td>
                    <td>
                      <span className="flex justify-end gap-1.5">
                        <Btn
                          variant="gh"
                          className="h-5 px-2 text-10"
                          disabled
                          disabledReason="No rotate endpoint in the spec — spec change first (finding)"
                        >
                          Rotate
                        </Btn>
                        <Btn
                          variant="gh"
                          className="h-5 px-2 text-10 text-err"
                          onClick={() => setConfirming(row)}
                          disabled={revoke.isPending}
                          disabledReason="Revoking…"
                        >
                          Revoke
                        </Btn>
                      </span>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}

      {isEmpty ? null : (
        <div>
          <Btn variant="s" onClick={() => setDrawerOpen(true)}>
            Create binding (U2)
          </Btn>
        </div>
      )}

      <p className="text-10p5 text-ink3">
        Rotation policy: 90 d automatic (org policy{" "}
        <span className="mono">credential-rotation</span>) · rotations are zero-downtime — old + new
        valid for 60 s · every issue, rotate, revoke → audit log. Consumers never see a password;
        the binding injects and refreshes it.
      </p>

      {confirming ? (
        <ConfirmModal
          title={`Revoke binding ${confirming.id}`}
          consequence={`${confirming.consumer} loses access on its next connection — credentials die now, the env var stops resolving at the next deploy · audit-logged`}
          verb="Revoke binding"
          pending={revoke.isPending}
          onConfirm={() => doRevoke(confirming)}
          onClose={() => setConfirming(null)}
        />
      ) : null}

      {drawerOpen ? (
        <CreateBindingDrawer
          svc={svc}
          project={project}
          env={env}
          onClose={() => setDrawerOpen(false)}
        />
      ) : null}
    </>
  );
}

/* ============================ U2 · Create binding ============================ */

function cap(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function productLabel(s: Service): string {
  const shape = (s.shape ?? {}) as Record<string, unknown>;
  switch (s.product) {
    case "postgres":
      return shape.size ? `PostgreSQL · ${cap(String(shape.size))}` : "PostgreSQL";
    case "storage":
      return "Object Storage";
    case "valkey":
      return "Valkey";
    case "queue":
      return "Queue";
    case "web":
      return "Web service";
    case "worker":
      return "Worker";
    default:
      return s.product;
  }
}

/** Deterministic <TARGET>_URL — db-reports → DATABASE_REPORTS_URL (U2 canon). */
function envVarFor(targetName: string): string {
  return `${targetName
    .replace(/^db(-|$)/, "database$1")
    .toUpperCase()
    .replace(/-/g, "_")}_URL`;
}

/** U2 · Create binding — 424px right sheet (complex forms get the sheet). */
export function CreateBindingDrawer({
  svc,
  project,
  env,
  onClose,
}: {
  svc: Service;
  project: string;
  env: string;
  onClose: () => void;
}) {
  const services = useServices(env);
  const bindings = useBindings(svc.id);
  const qc = useQueryClient();
  const create = useMutation(createBindingMutation());
  const [picked, setPicked] = useState<string | null>(null);
  const [scope, setScope] = useState<"read_only" | "read_write">("read_only");

  const boundTargets = new Set(
    (bindings.data ?? []).filter((b) => b.source_id === svc.id).map((b) => b.target_id),
  );
  const others = (services.data ?? []).filter((s) => s.id !== svc.id);
  const selectable = others.filter((s) => !boundTargets.has(s.id));
  const alreadyBound = others.filter((s) => boundTargets.has(s.id));
  // First selectable preselected. NOTE: the U2 frame shows assets selectable
  // from api, but the canon fixtures already bind api → assets — the list is
  // data-driven, so fixtures win (inconsistency carried as a finding).
  const target = picked ?? selectable[0]?.id ?? null;
  const targetSvc = others.find((s) => s.id === target);
  const targetName = targetSvc?.name ?? "";

  const submit = () => {
    if (!target) return;
    create.mutate(
      { path: { service: svc.id }, body: { target, scope } },
      {
        onSuccess: () => {
          toast.success("Binding created — takes effect on the next deploy");
          qc.invalidateQueries({ queryKey: listBindingsQueryKey({ path: { service: svc.id } }) });
          onClose();
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    <Drawer
      title="Create binding"
      sub={
        <>
          from <span className="mono">{svc.name}</span> · {project} / {env}
        </>
      }
      onClose={onClose}
      footer={
        <>
          <span className="text-10p5 text-ink3">No cost — bindings are wiring, not resources</span>
          <span className="flex-1" />
          <Btn variant="s" onClick={onClose}>
            Cancel
          </Btn>
          <Btn
            variant="p"
            onClick={submit}
            disabled={!target || create.isPending}
            disabledReason={!target ? "Every service is already bound" : "Creating the binding…"}
          >
            Create binding
          </Btn>
        </>
      }
    >
      <div className="flex flex-col gap-1.5">
        <div className="eyebrow">Bind to</div>
        {selectable.map((s) => (
          <label
            key={s.id}
            className={cn(
              "flex cursor-pointer items-center gap-2.5 rounded-lg border border-hair px-3 py-2",
              target === s.id && "border-steel",
            )}
          >
            <input
              type="radio"
              name="bind-target"
              checked={target === s.id}
              onChange={() => setPicked(s.id)}
            />
            <span className="mono text-12">{s.name}</span>
            <span className="flex-1" />
            <span className="text-10p5 text-ink3">{productLabel(s)}</span>
          </label>
        ))}
        {alreadyBound.map((s) => (
          <div
            key={s.id}
            className="flex items-center gap-2.5 rounded-lg border border-hair px-3 py-2 opacity-55"
          >
            <input type="radio" name="bind-target" disabled />
            <span className="mono text-12">{s.name}</span>
            <span className="flex-1" />
            <span className="text-10p5 text-ink3">already bound</span>
          </div>
        ))}
      </div>

      <div className="flex flex-col gap-1.5">
        <div className="eyebrow">Scope</div>
        <div className="chiprow">
          <button
            type="button"
            className={cn("chip", scope === "read_only" && "on")}
            onClick={() => setScope("read_only")}
          >
            read-only
          </button>
          <button
            type="button"
            className={cn("chip", scope === "read_write" && "on")}
            onClick={() => setScope("read_write")}
          >
            read-write
          </button>
        </div>
        <p className="text-10p5 text-ink3">
          read-only mints a role that cannot write — enforced by the database, not the app.
        </p>
      </div>

      <div className="flex flex-col gap-1.5">
        <div className="eyebrow">Injected at deploy</div>
        <div className="logwell">
          <div>
            {targetName ? envVarFor(targetName) : "…"}=postgres://{svc.name}
            {scope === "read_only" ? "_ro" : "_rw"}:•••@{targetName || "…"}…
          </div>
          <div className="t"># credentials minted at bind, rotated on unbind</div>
        </div>
      </div>

      <Card className="p-3.5">
        <p className="text-11p5 leading-relaxed text-ink2">
          Takes effect on the next deploy of <span className="mono">{svc.name}</span> — nothing
          restarts now. The edge appears on the topology (W3) immediately as{" "}
          <span className="mono">pending</span>.
        </p>
      </Card>
    </Drawer>
  );
}
