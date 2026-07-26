import { type ReactNode, type RefObject, useEffect, useRef } from "react";
import { cn } from "@/lib/utils";
import { Icon } from "./icon";

/**
 * Shared overlay mechanics (01-design-system): every overlay closes on Esc
 * non-destructively, traps Tab inside its panel, and restores focus to the
 * opener on close. The scrim is the --scrim token. Recipes: modal 460px
 * centered · typed-confirm 480px · drawer 424px right sheet with sticky
 * header/footer. Popovers and the assistant side-panel reuse useOverlay
 * without a Scrim.
 */

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/**
 * Esc-to-close (document-level, works regardless of focus) + Tab trap +
 * focus restore. Pass `onClose: undefined` for non-dismissible overlays
 * (e.g. the reveal-once token modal) — the trap still applies.
 */
export function useOverlay(
  panelRef: RefObject<HTMLElement | null>,
  onClose?: () => void,
  opts?: { trap?: boolean },
) {
  // Ref so inline-closure onClose props don't re-run the effect (which would
  // steal focus back on every render).
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  const hasClose = onClose !== undefined;
  const trap = opts?.trap !== false;

  useEffect(() => {
    const node = panelRef.current;
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    // Initial focus: an autoFocus child already has it; otherwise the first
    // focusable, otherwise the panel itself.
    if (node && !node.contains(document.activeElement)) {
      const target =
        node.querySelector<HTMLElement>("[autofocus]") ??
        node.querySelector<HTMLElement>(FOCUSABLE) ??
        node;
      target.focus();
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && hasClose) {
        e.stopPropagation();
        closeRef.current?.();
        return;
      }
      if (trap && e.key === "Tab" && node) {
        const els = Array.from(node.querySelectorAll<HTMLElement>(FOCUSABLE));
        const first = els[0];
        const last = els[els.length - 1];
        if (!first || !last) {
          e.preventDefault();
          return;
        }
        const active = document.activeElement;
        if (!node.contains(active)) {
          e.preventDefault();
          first.focus();
        } else if (e.shiftKey && active === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && active === last) {
          e.preventDefault();
          first.focus();
        }
      }
    };
    document.addEventListener("keydown", onKey, true);
    return () => {
      document.removeEventListener("keydown", onKey, true);
      opener?.focus();
    };
  }, [panelRef, hasClose, trap]);
}

/** Full-viewport scrim (--scrim token). Click dismisses when onDismiss given. */
export function Scrim({
  onDismiss,
  className,
  children,
}: {
  onDismiss?: () => void;
  className?: string;
  children: ReactNode;
}) {
  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: scrim click mirrors Esc, which useOverlay handles
    // biome-ignore lint/a11y/useKeyWithClickEvents: Esc handled at the document level by useOverlay
    <div
      className={cn("fixed inset-0 z-40 bg-scrim", className)}
      role="presentation"
      onClick={onDismiss}
    >
      {children}
    </div>
  );
}

/**
 * Centered modal — 460px, or 480px for typed-confirm (pass width).
 * `dismissible: false` removes Esc/scrim-dismiss for reveal-once flows;
 * children supply the header, body, and footer rows (flex-col gap-4).
 */
export function Modal({
  label,
  onClose,
  width = 460,
  dismissible = true,
  className,
  children,
}: {
  label: string;
  onClose: () => void;
  width?: 460 | 480;
  dismissible?: boolean;
  className?: string;
  children: ReactNode;
}) {
  const panel = useRef<HTMLDivElement>(null);
  useOverlay(panel, dismissible ? onClose : undefined);
  return (
    <Scrim
      onDismiss={dismissible ? onClose : undefined}
      className="flex items-center justify-center"
    >
      {/* biome-ignore lint/a11y/useKeyWithClickEvents: click-stop only, so the scrim doesn't dismiss */}
      {/* biome-ignore lint/a11y/noStaticElementInteractions: click-stop only */}
      <div
        ref={panel}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-label={label}
        style={{ width }}
        className={cn(
          "flex max-h-[85vh] flex-col gap-4 overflow-y-auto rounded-xl border border-hair bg-surface p-5 shadow-e2 outline-none",
          className,
        )}
        onClick={(e) => e.stopPropagation()}
      >
        {children}
      </div>
    </Scrim>
  );
}

/** Modal header pair — title + one-line consequence sub. */
export function ModalHead({ title, sub }: { title: ReactNode; sub?: ReactNode }) {
  return (
    <div>
      <div className="text-14 font-semibold">{title}</div>
      {sub ? <div className="hsub">{sub}</div> : null}
    </div>
  );
}

/**
 * Right-sheet drawer — 424px, sticky header (title · mono sub · ✕) and
 * sticky footer; the body scrolls. Esc, scrim click, and ✕ all close.
 */
export function Drawer({
  title,
  sub,
  onClose,
  footer,
  label,
  children,
}: {
  title: ReactNode;
  sub?: ReactNode;
  onClose: () => void;
  footer?: ReactNode;
  label?: string;
  children: ReactNode;
}) {
  const panel = useRef<HTMLDivElement>(null);
  useOverlay(panel, onClose);
  return (
    <Scrim onDismiss={onClose}>
      {/* biome-ignore lint/a11y/useKeyWithClickEvents: click-stop only, so the scrim doesn't dismiss */}
      {/* biome-ignore lint/a11y/noStaticElementInteractions: click-stop only */}
      <div
        ref={panel}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-label={label ?? (typeof title === "string" ? title : undefined)}
        className="absolute inset-y-0 right-0 flex w-[424px] flex-col border-hair border-l bg-surface shadow-e2 outline-none"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start gap-3 border-hair border-b px-5 py-4">
          <div className="min-w-0">
            <div className="text-14 font-semibold">{title}</div>
            {sub ? <div className="mono mt-0.5 text-10p5 text-ink3">{sub}</div> : null}
          </div>
          <button
            type="button"
            className="ml-auto shrink-0 p-0.5 text-ink3 hover:text-ink1"
            onClick={onClose}
            aria-label="Close"
          >
            <Icon id="s-x" className="h-3.5 w-3.5" />
          </button>
        </div>
        <div className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">{children}</div>
        {footer ? (
          <div className="flex items-center gap-2 border-hair border-t px-5 py-3.5">{footer}</div>
        ) : null}
      </div>
    </Scrim>
  );
}
