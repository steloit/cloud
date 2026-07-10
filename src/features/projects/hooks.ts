import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createProjectMutation,
  getProjectOptions,
  listEnvironmentsOptions,
  listProjectsOptions,
  listProjectsQueryKey,
} from "@/lib/api";

export function useProjects(org: string) {
  return useQuery({ ...listProjectsOptions({ path: { org } }), select: (r) => r.data ?? [] });
}

export function useProject(project: string) {
  return useQuery(getProjectOptions({ path: { project } }));
}

export function useEnvironments(project: string) {
  return useQuery({
    ...listEnvironmentsOptions({ path: { project } }),
    select: (r) => r.data ?? [],
  });
}

export function useCreateProject(org: string) {
  const queryClient = useQueryClient();
  return useMutation({
    ...createProjectMutation(),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: listProjectsQueryKey({ path: { org } }) }),
  });
}
