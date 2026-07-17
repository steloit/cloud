import type { Service } from "@/lib/api";

/**
 * X1's synthetic service object — client-side only, never served by canon
 * mode (adding it to the world would break the 7-services-=-$208 invariant;
 * stated finding). It exists so the gateway rides the standard service-page
 * architecture: SnavProduct, the overview dispatch, and the tab dispatchers
 * all treat it like any other Service.
 */
export const GATEWAY_SERVICE: Service = {
  id: "svc_gateway_x1",
  env_id: "env_prod",
  name: "gateway",
  product: "ai-gateway",
  status: "ready",
  shape: { models: 4, routes: 3, cache: "semantic" },
  region: "aws/ap-south-1",
  monthly_estimate_cents: 3400,
};

/** Standard service-param resolution + the gateway exemplar fallback. */
export function resolveService(
  services: Service[] | undefined,
  param: string,
): Service | undefined {
  const found = services?.find((s) => s.name === param || s.id === param);
  if (found) return found;
  if (param === "gateway" || param === "ai-gateway") return GATEWAY_SERVICE;
  return undefined;
}
