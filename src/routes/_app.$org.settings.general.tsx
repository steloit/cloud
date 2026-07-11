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
import { RenameModal } from "@/features/common/rename-modal";
import { useOrgs } from "@/features/org/hooks";
import { deleteOrgMutation, errorMessage, listMyOrgsQueryKey, updateOrgMutation } from "@/lib/api";

/** G5 · Organization · General — the boundary of identity, billing and policy. */

function OrgGeneralPage() {
  const { org } = Route.useParams();
  const orgs = useOrgs();
  const orgRecord = orgs.data?.find((o) => o.slug === org || o.id === org);
  const orgName = orgRecord?.name ?? "Acme";

  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const rename = useMutation(updateOrgMutation());
  const del = useMutation(deleteOrgMutation());

  const [renameOpen, setRenameOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  return (
    <>
      <SnavSettings
        org={org}
        orgName={orgRecord?.name ?? org}
        project="ecommerce"
        active="o-general"
      />
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Organization · General"
            sub="Acme · Business plan · the boundary of identity, billing and policy"
          />

          <Card className="flex max-w-[640px] flex-col gap-4 p-4">
            <div>
              <Flabel htmlFor="org-name">Organization name</Flabel>
              {/* Pass-4 (P1 finding): the name field looked editable but was inert —
                  now honest readOnly, with rename as an explicit overlay verb. */}
              <div className="flex items-center gap-2.5">
                <span className="flex-1">
                  <Inp id="org-name" defaultValue="Acme" readOnly />
                </span>
                <Btn variant="s" onClick={() => setRenameOpen(true)}>
                  Rename…
                </Btn>
              </div>
            </div>
            <div>
              <Flabel htmlFor="org-slug">Slug</Flabel>
              <Inp id="org-slug" className="mono" defaultValue="acme" readOnly />
            </div>
            <div>
              <Flabel htmlFor="org-region">Default home region</Flabel>
              <Inp id="org-region" className="mono" defaultValue="aws · ap-south-1" readOnly />
              <div className="mt-1.5 text-10p5 text-ink3">
                prefill for new environments — never a constraint
              </div>
            </div>
          </Card>

          <Card className="flex max-w-[640px] flex-col gap-3 p-4">
            <Eyebrow>Ownership</Eyebrow>
            <div className="text-12 text-ink2">
              asha is the owner. Transfer requires the new owner to accept — it never happens to
              someone.
            </div>
            <div>
              <Btn variant="s" disabled disabledReason="Ownership transfer lands in Phase 4">
                Transfer ownership…
              </Btn>
            </div>
          </Card>

          <Card className="flex max-w-[640px] flex-col gap-3 border-err/45 p-4">
            <Eyebrow className="text-err">Danger zone</Eyebrow>
            {/* Pass-4: the frame's disabled verb + "blocked" pill become a live verb
                behind the 480 typed confirm — B6 grammar moves into the overlay.
                A real block now arrives as a 409 Problem (DeleteOrgErrors) and is
                toasted with its blocking reasons, not painted on statically. */}
            <div className="flex items-center gap-2.5">
              <Btn variant="dgr" onClick={() => setDeleteOpen(true)}>
                Delete Acme…
              </Btn>
            </div>
            <div className="text-10p5 text-ink3">
              delete or transfer every project first; the final invoice closes the account, and
              audit history is exported before erasure
            </div>
          </Card>
        </div>
      </main>
      {renameOpen ? (
        <RenameModal
          entity="organization"
          current={orgName}
          consequence="renames the org everywhere — URLs keep working via the old slug for 30 days"
          pending={rename.isPending}
          onClose={() => setRenameOpen(false)}
          onSave={(name) =>
            rename.mutate(
              { path: { org }, body: { name } },
              {
                onSuccess: () => {
                  // Echo mutation (the DB8 precedent): PATCH succeeds for the
                  // contract's sake; fixtures win on refetch — canon stays Acme.
                  toast.success(`Renamed to ${name} — the old slug keeps working for 30 days`);
                  queryClient.invalidateQueries({ queryKey: listMyOrgsQueryKey() });
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
          title={`Delete ${orgName}`}
          consequence="every project, service and byte goes with it — deletion starts a 7-day grace window, owners are emailed"
          expected={orgName}
          verb="Delete organization"
          pending={del.isPending}
          onClose={() => setDeleteOpen(false)}
          onConfirm={() =>
            del.mutate(
              { path: { org } },
              {
                onSuccess: () => {
                  // Echo mutation (the DB8 precedent): DELETE 204s; fixtures win
                  // on refetch — canon's org_acme survives the grace window.
                  toast.success(
                    `${orgName} is being deleted — 7-day grace window started, owners emailed`,
                  );
                  navigate({ to: "/" });
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

export const Route = createFileRoute("/_app/$org/settings/general")({
  component: OrgGeneralPage,
});
