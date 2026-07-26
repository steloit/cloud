import type { Service } from "@/lib/api";

/** Standard service-param resolution (name or id). */
export function resolveService(
  services: Service[] | undefined,
  param: string,
): Service | undefined {
  return services?.find((s) => s.name === param || s.id === param);
}
