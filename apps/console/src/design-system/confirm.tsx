import { type ReactNode, useState } from "react";
import { Btn } from "./btn";
import { Flabel, Inp } from "./inp";
import { Modal, ModalHead } from "./overlay";

/**
 * The two confirmation recipes (01-design-system overlays): a 460px confirm
 * modal for destructive actions, and the 480px typed-confirm for
 * irreversible-with-name ones. One idiom app-wide — no more two-click armed
 * buttons for census-listed destructive verbs.
 */

export function ConfirmModal({
  title,
  consequence,
  verb,
  tone = "danger",
  onConfirm,
  onClose,
  pending,
  /** Present = the verb itself is gated (B6): confirm stays disabled with this reason. */
  gatedReason,
  children,
}: {
  title: string;
  consequence: ReactNode;
  verb: string;
  tone?: "danger" | "primary";
  onConfirm: () => void;
  onClose: () => void;
  pending?: boolean;
  gatedReason?: string;
  children?: ReactNode;
}) {
  return (
    <Modal label={title} onClose={onClose}>
      <ModalHead title={title} sub={consequence} />
      {children}
      <div className="flex justify-end gap-2">
        <Btn variant="s" onClick={onClose}>
          Cancel
        </Btn>
        <Btn
          variant={tone === "danger" ? "dgr" : "p"}
          onClick={onConfirm}
          disabled={pending || Boolean(gatedReason)}
          disabledReason={gatedReason ?? "Working…"}
        >
          {verb}
        </Btn>
      </div>
    </Modal>
  );
}

export function TypedConfirmModal({
  title,
  consequence,
  expected,
  verb,
  onConfirm,
  onClose,
  pending,
  gatedReason,
  children,
}: {
  title: string;
  consequence: ReactNode;
  /** The exact string the user must type — a name, never a slug of one. */
  expected: string;
  verb: string;
  onConfirm: () => void;
  onClose: () => void;
  pending?: boolean;
  /** Present = the verb is gated even when typed (honest reason ladder). */
  gatedReason?: string;
  children?: ReactNode;
}) {
  const [typed, setTyped] = useState("");
  const match = typed === expected;
  const reason = !match
    ? `Type ${expected} to confirm`
    : (gatedReason ?? (pending ? "Working…" : undefined));
  return (
    <Modal label={title} onClose={onClose} width={480}>
      <ModalHead title={title} sub={consequence} />
      {children}
      <div>
        <Flabel htmlFor="typed-confirm">
          Type <span className="mono font-semibold">{expected}</span> to confirm
        </Flabel>
        <Inp
          id="typed-confirm"
          className="mono"
          value={typed}
          autoFocus
          onChange={(e) => setTyped(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && match && !pending && !gatedReason) onConfirm();
          }}
        />
      </div>
      <div className="flex justify-end gap-2">
        <Btn variant="s" onClick={onClose}>
          Cancel
        </Btn>
        <Btn
          variant="dgr"
          onClick={onConfirm}
          disabled={!match || pending || Boolean(gatedReason)}
          disabledReason={reason}
        >
          {verb}
        </Btn>
      </div>
    </Modal>
  );
}
