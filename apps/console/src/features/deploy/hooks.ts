import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { rollbackDeploymentMutation } from "@/lib/api";

/**
 * Deploy-domain mutations. Reads stay in features/services/hooks
 * (useDeployments) — env-as-filter keys already embed {env} there.
 */
export function useRollback() {
  return useMutation({
    ...rollbackDeploymentMutation(),
    onSuccess: () => toast.success("Rollback started — redeploy of the previous image (<60 s)"),
  });
}
