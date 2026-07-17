import { useState } from "react";
import { Btn } from "@/design-system/btn";
import { Flabel, Inp } from "@/design-system/inp";
import { Modal, ModalHead } from "@/design-system/overlay";

/**
 * The one rename grammar (overlay census): current name pre-filled, a
 * consequence line stating what the rename touches (URLs, bindings, nothing
 * silent), Save disabled until the name actually changes.
 */
export function RenameModal({
  entity,
  current,
  consequence,
  onSave,
  onClose,
  pending,
  /** Present = rename is gated (B6): Save stays disabled with this reason. */
  gatedReason,
}: {
  /** "organization" · "project" · "service" · "template" — lowercase noun. */
  entity: string;
  current: string;
  consequence: string;
  onSave: (name: string) => void;
  onClose: () => void;
  pending?: boolean;
  gatedReason?: string;
}) {
  const [name, setName] = useState(current);
  const changed = name.trim() !== "" && name.trim() !== current;
  const reason = !changed
    ? "Enter a different name"
    : (gatedReason ?? (pending ? "Renaming…" : undefined));
  return (
    <Modal label={`Rename ${entity}`} onClose={onClose}>
      <ModalHead title={`Rename ${entity}`} sub={consequence} />
      <div>
        <Flabel htmlFor="rename-input">Name</Flabel>
        <Inp
          id="rename-input"
          value={name}
          autoFocus
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && changed && !pending && !gatedReason) onSave(name.trim());
          }}
        />
      </div>
      <div className="flex justify-end gap-2">
        <Btn variant="s" onClick={onClose}>
          Cancel
        </Btn>
        <Btn
          variant="p"
          onClick={() => onSave(name.trim())}
          disabled={!changed || pending || Boolean(gatedReason)}
          disabledReason={reason}
        >
          Save
        </Btn>
      </div>
    </Modal>
  );
}
