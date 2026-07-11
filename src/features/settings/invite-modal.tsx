import { useState } from "react";
import { toast } from "sonner";
import { Btn } from "@/design-system/btn";
import { Flabel, Inp } from "@/design-system/inp";
import { useCreateInvite } from "@/features/invites/hooks";
import { errorMessage } from "@/lib/api";

type InviteRole = "admin" | "developer" | "billing";

const ROLE_LABELS: Record<InviteRole, string> = {
  admin: "Admin",
  developer: "Developer",
  billing: "Billing",
};

/** U1 · Invite member — 460px modal over G6. */
export function InviteModal({ org, onClose }: { org: string; onClose: () => void }) {
  const createInvite = useCreateInvite();
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<InviteRole>("developer");

  const send = () => {
    if (!email.includes("@")) return;
    createInvite.mutate(
      { path: { org }, body: { email, role } },
      {
        onSuccess: () => {
          toast.success(`Invite sent to ${email} — expires in 7 days`);
          onClose();
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: scrim dismiss mirrors Esc
    // biome-ignore lint/a11y/useKeyWithClickEvents: Esc handled on the email input
    <div
      className="fixed inset-0 z-40 flex items-center justify-center bg-[rgba(6,9,12,0.44)]"
      onClick={onClose}
    >
      {/* biome-ignore lint/a11y/useKeyWithClickEvents: click-stop only, so the scrim doesn't dismiss; Esc handled on the input */}
      <div
        className="flex w-[460px] flex-col gap-4 rounded-xl border border-hair bg-surface p-5 shadow-e2"
        role="dialog"
        aria-label="Invite member"
        onClick={(e) => e.stopPropagation()}
      >
        <div>
          <div className="text-[14px] font-semibold">Invite member</div>
          <div className="hsub">
            Joins the organization with a role — invites expire after 7 days.
          </div>
        </div>
        <div>
          <Flabel htmlFor="invite-email">Email</Flabel>
          <Inp
            id="invite-email"
            type="email"
            placeholder="name@company.com"
            value={email}
            autoFocus
            onChange={(e) => setEmail(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Escape") onClose();
              if (e.key === "Enter") send();
            }}
          />
        </div>
        <div>
          <Flabel htmlFor="invite-role">Role</Flabel>
          <select
            id="invite-role"
            className="inp w-full"
            value={role}
            onChange={(e) => setRole(e.target.value as InviteRole)}
          >
            {(Object.keys(ROLE_LABELS) as InviteRole[]).map((r) => (
              <option key={r} value={r}>
                {ROLE_LABELS[r]}
              </option>
            ))}
          </select>
        </div>
        <div className="text-[10.5px] text-ink3">
          13 of 20 seats after this invite — seats beyond 20 add $7/mo each (B7).
        </div>
        <div className="flex justify-end gap-2">
          <Btn variant="s" onClick={onClose}>
            Cancel
          </Btn>
          <Btn
            variant="p"
            onClick={send}
            disabled={createInvite.isPending}
            disabledReason="Sending the invite…"
          >
            Send invite
          </Btn>
        </div>
      </div>
    </div>
  );
}
