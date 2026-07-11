import { useMutation, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { SnavSettings } from "@/app/shell/snav-settings";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { TypedConfirmModal } from "@/design-system/confirm";
import { Eyebrow } from "@/design-system/eyebrow";
import { Flabel, Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { RenameModal } from "@/features/common/rename-modal";
import { useOrgs } from "@/features/org/hooks";
import {
  deleteProjectMutation,
  errorMessage,
  listProjectsQueryKey,
  updateProjectMutation,
} from "@/lib/api";

/** G1 · Project · General — identity, transfer, danger zone. */

function ProjectGeneralPage() {
  const { org, project } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);

  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const rename = useMutation(updateProjectMutation());
  const del = useMutation(deleteProjectMutation());

  const [renameOpen, setRenameOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project={project}
        active="p-general"
      />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Project · General"
            sub="ecommerce · created from the store template · 7 months ago"
          />

          <Card className="flex max-w-[640px] flex-col gap-4 p-4">
            <div>
              <Flabel htmlFor="prj-name">Project name</Flabel>
              {/* Pass-4 (P1 finding): the name field looked editable but was inert —
                  now honest readOnly, with rename as an explicit overlay verb. */}
              <div className="flex items-center gap-2.5">
                <span className="flex-1">
                  <Inp id="prj-name" defaultValue="ecommerce" readOnly />
                </span>
                <Btn variant="s" onClick={() => setRenameOpen(true)}>
                  Rename…
                </Btn>
              </div>
            </div>
            <div>
              <Flabel htmlFor="prj-slug">Slug</Flabel>
              <div className="flex items-center gap-2.5">
                <span className="flex-1">
                  <Inp id="prj-slug" className="mono" defaultValue="acme/ecommerce" readOnly />
                </span>
                <Pill tone="mut">renaming updates URLs · old slugs redirect 90 d</Pill>
              </div>
            </div>
            <div>
              <Flabel htmlFor="prj-defenv">Default environment</Flabel>
              <Inp id="prj-defenv" className="mono" defaultValue="production" readOnly />
            </div>
          </Card>

          <Card className="flex max-w-[640px] flex-col gap-3 p-4">
            <Eyebrow>Transfer</Eyebrow>
            <div className="text-12 text-ink2">
              Moves services, environments, history and cost — bindings and credentials survive;
              billing switches at the next invoice boundary.
            </div>
            <div>
              <Btn variant="s" disabled disabledReason="Project transfer lands in Phase 4">
                Transfer…
              </Btn>
            </div>
          </Card>

          <Card className="flex max-w-[640px] flex-col gap-3 border-err/45 p-4">
            <Eyebrow className="text-err">Danger zone</Eyebrow>
            {/* G1 frame truth: canon ecommerce's delete is dependency-blocked,
                and "the button says why" — the typed-confirm modal below ships
                as the unblocked path and opens only once the blockers clear
                (never, in canon). */}
            <div className="flex items-center gap-2.5">
              <Btn
                variant="dgr"
                disabled
                disabledReason="blocked — 7 services across 3 environments"
                onClick={() => setDeleteOpen(true)}
              >
                Delete ecommerce…
              </Btn>
              <Pill tone="err">blocked — 7 services across 3 environments</Pill>
            </div>
            {/* Finding: the frame says "typing the project slug", but the typed-confirm
                grammar expects the exact name, never a slug of one — name wins. */}
            <div className="text-10p5 text-ink3">
              deletion spins services down in dependency order, keeps final snapshots 30 d, and
              requires typing the project slug
            </div>
          </Card>
        </div>
      </main>
      {renameOpen ? (
        <RenameModal
          entity="project"
          current={project}
          consequence="renames the project everywhere — deploy URLs and binding env-var names re-derive; old slugs redirect 90 d"
          pending={rename.isPending}
          onClose={() => setRenameOpen(false)}
          onSave={(name) =>
            rename.mutate(
              { path: { project }, body: { name } },
              {
                onSuccess: () => {
                  // Echo mutation (the DB8 precedent): PATCH succeeds for the
                  // contract's sake; fixtures win on refetch — canon stays ecommerce.
                  toast.success(
                    `Renamed to ${name} — deploy URLs and binding env-var names re-derive; old slugs redirect 90 d`,
                  );
                  queryClient.invalidateQueries({
                    queryKey: listProjectsQueryKey({ path: { org } }),
                  });
                  setRenameOpen(false);
                },
                onError: (err) => toast.error(errorMessage(err)),
              },
            )
          }
        />
      ) : null}
      {deleteOpen ? (
        <TypedConfirmModal
          title={`Delete ${project}`}
          consequence="services, environments and their data go with it — deletion starts a 7-day grace window"
          expected={project}
          verb="Delete project"
          pending={del.isPending}
          onClose={() => setDeleteOpen(false)}
          onConfirm={() =>
            del.mutate(
              { path: { project } },
              {
                onSuccess: () => {
                  // Echo mutation (the DB8 precedent): DELETE 204s; fixtures win
                  // on refetch — canon's ecommerce survives the grace window.
                  toast.success(`${project} is being deleted — 7-day grace window started`);
                  navigate({ to: "/$org", params: { org } });
                },
                onError: (err) => toast.error(errorMessage(err)),
              },
            )
          }
        />
      ) : null}
    </>
  );
}

export const Route = createFileRoute("/_app/$org/$project/settings/general")({
  component: ProjectGeneralPage,
});
