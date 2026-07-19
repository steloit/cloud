// @steloit/canon — the one demo world (ADR-026) and its arithmetic invariants,
// as the single import point for tests (canon-testing pack). The console and
// any TS layer assert the SAME equations the Go estimate/invoice layers do
// (services/api/internal/canon), against the SAME numbers — a canon change that
// breaks the math fails every layer at once.
//
// fixtures.json is a copy synced from docs/product/19-canon/ by `make gen-ts`;
// CI's `git diff --exit-code` fails the build if it drifts from source.

import fixtures from "./fixtures.json" with { type: "json" };

export { fixtures };

// Money is integer cents end-to-end (ADR-025).
interface Project {
  id: string;
  name: string;
  monthly_cost_cents: number;
}
interface Environment {
  id: string;
  name: string;
  monthly_cost_cents: number;
}
interface Service {
  name: string;
  monthly_estimate_cents: number;
}
interface Billing {
  resources_cents: number;
  plan_fee_cents: number;
  forecast_cents: number;
}

// The fixtures' top-level keys are annotated (e.g. "projects (Project,
// org_acme)"), so sections are matched by prefix — never by a brittle exact key.
function section<T>(prefix: string): T {
  const doc = fixtures as Record<string, unknown>;
  const key = Object.keys(doc).find((k) => k.startsWith(prefix));
  if (!key) throw new Error(`canon: no fixtures section "${prefix}"`);
  return doc[key] as T;
}

export const projects = (): Project[] => section("projects");
export const environments = (): Environment[] => section("environments");
export const services = (): Service[] => section("services");
export const billing = (): Billing => section("billing_overview");

export const ecommerceProjectCents = (): number => {
  const p = projects().find((p) => p.name === "ecommerce");
  if (!p) throw new Error("canon: ecommerce project not found");
  return p.monthly_cost_cents;
};

const sum = (ns: number[]): number => ns.reduce((a, b) => a + b, 0);

/**
 * assertArithmetic verifies the four canon.md §invariants against the fixtures,
 * throwing on the first violation. The console and Go layers run the identical
 * equations:
 *
 *   Σ ecommerce services     == ecommerce project total   (61+22+58+24+22+21 = 208)
 *   Σ ecommerce environments == ecommerce project total   (199.10+6.70+2.20 = 208)
 *   Σ project totals         == org resources             (208+96+41+38+0 = 383)
 *   resources + plan fee     == org total                 (383+99 = 482)
 */
export function assertArithmetic(doc: unknown = fixtures): void {
  // Allow callers (e.g. the console over its own fixtures copy) to pass a
  // document; default to this package's synced copy.
  const pick = <T>(prefix: string): T => {
    const d = doc as Record<string, unknown>;
    const key = Object.keys(d).find((k) => k.startsWith(prefix));
    if (!key) throw new Error(`canon: no fixtures section "${prefix}"`);
    return d[key] as T;
  };
  const prj = pick<Project[]>("projects");
  const env = pick<Environment[]>("environments");
  const svc = pick<Service[]>("services");
  const bill = pick<Billing>("billing_overview");

  const proj = prj.find((p) => p.name === "ecommerce")?.monthly_cost_cents;
  if (proj == null) throw new Error("canon: ecommerce project not found");

  const svcSum = sum(svc.map((s) => s.monthly_estimate_cents));
  if (svcSum !== proj) throw new Error(`Σ services ${svcSum} ≠ ecommerce project ${proj}`);

  const envSum = sum(env.map((e) => e.monthly_cost_cents));
  if (envSum !== proj) throw new Error(`Σ environments ${envSum} ≠ ecommerce project ${proj}`);

  const projSum = sum(prj.map((p) => p.monthly_cost_cents));
  if (projSum !== bill.resources_cents)
    throw new Error(`Σ projects ${projSum} ≠ org resources ${bill.resources_cents}`);

  const total = bill.resources_cents + bill.plan_fee_cents;
  if (total !== bill.forecast_cents)
    throw new Error(
      `resources ${bill.resources_cents} + plan ${bill.plan_fee_cents} = ${total} ≠ org total ${bill.forecast_cents}`,
    );
}
