/**
 * Canon-mode env-key resolution: the real API addresses environments by id
 * (env_prod), but the console's ?env= carries the display name — and env
 * names collide across projects (both ecommerce and internal-tools have a
 * "production"). Callers with project context resolve through this helper.
 */
export function resolveEnvKey(project: string | undefined, env: string): string {
  if (project === "internal-tools" && env === "production") return "env_it_prod";
  return env;
}
