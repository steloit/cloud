import { ConfirmModal } from "@/design-system/confirm";
import { useRollback } from "@/features/deploy/hooks";

/**
 * The one rollback idiom (overlay census): every rollback affordance — DP1's
 * row action, D19's header button and per-row history actions — opens this
 * confirm instead of arming in place or firing bare. One consequence line,
 * one verb, app-wide.
 */
export function RollbackConfirm({
  target,
  dep,
  onClose,
}: {
  /** The deploy being restored, e.g. "#141" — titles the modal. */
  target: string;
  /** The deployment id the rollback POST addresses. */
  dep: string;
  onClose: () => void;
}) {
  const rollback = useRollback();
  return (
    <ConfirmModal
      title={`Roll back to ${target}`}
      consequence="replays the last known-good image — instances replace one at a time behind the health check, ~90 s, no data migration runs backwards"
      verb="Roll back"
      pending={rollback.isPending}
      onClose={onClose}
      onConfirm={() => rollback.mutate({ path: { dep } }, { onSettled: onClose })}
    />
  );
}
