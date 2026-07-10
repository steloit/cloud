import { describe, expect, it } from "vitest";
import fixtures from "../src/lib/canon/fixtures.json";

/**
 * 16-qa arithmetic invariants — imported from 19-canon/fixtures.json, never
 * retyped: money arithmetic must reconcile everywhere it appears.
 */

const raw = fixtures as Record<string, unknown>;

function section<T>(prefix: string): T {
  const key = Object.keys(raw).find((k) => k === prefix || k.startsWith(`${prefix} (`));
  if (!key) throw new Error(`missing fixtures section ${prefix}`);
  return raw[key] as T;
}

interface Costed {
  monthly_cost_cents?: number;
  monthly_estimate_cents?: number;
}

describe("canon arithmetic invariants (16-qa)", () => {
  it("ecommerce services sum to the project total: 61+22+58+24+22+9+12 = 208", () => {
    const services = section<Costed[]>("services");
    const sum = services.reduce((acc, s) => acc + (s.monthly_estimate_cents ?? 0), 0);
    expect(sum).toBe(20800);
  });

  it("environment costs sum to the project total: 199.10+6.70+2.20 = 208", () => {
    const envs = section<Costed[]>("environments");
    const sum = envs.reduce((acc, e) => acc + (e.monthly_cost_cents ?? 0), 0);
    expect(sum).toBe(20800);
  });

  it("project costs sum to org resources: 208+96+41+38+0 = 383", () => {
    const projects = section<Costed[]>("projects");
    const sum = projects.reduce((acc, p) => acc + (p.monthly_cost_cents ?? 0), 0);
    expect(sum).toBe(38300);
  });

  it("org total = resources + plan fee: 383+99 = 482", () => {
    const billing = section<{
      resources_cents: number;
      plan_fee_cents: number;
      forecast_cents: number;
    }>("billing_overview");
    expect(billing.resources_cents + billing.plan_fee_cents).toBe(billing.forecast_cents);
    expect(billing.forecast_cents).toBe(48200);
  });

  it("billing by_project forecasts reconcile with project costs", () => {
    const billing = section<{ by_project: Array<{ name: string; forecast_cents: number }> }>(
      "billing_overview",
    );
    const projects = section<Array<{ name: string; monthly_cost_cents?: number }>>("projects");
    for (const line of billing.by_project) {
      const project = projects.find((p) => p.name === line.name);
      expect(project?.monthly_cost_cents).toBe(line.forecast_cents);
    }
  });
});
