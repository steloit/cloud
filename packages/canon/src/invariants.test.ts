import { describe, expect, it } from "vitest";
import fixturesJson from "./fixtures.json" with { type: "json" };
import { assertArithmetic, billing, ecommerceProjectCents, services } from "./index.js";

// The TS layer of the three-layer arithmetic invariant (Q2). The console
// imports assertArithmetic and runs it over its own fixtures copy; here we run
// it over the canonical package copy. Both assert the same equations the Go
// estimate/invoice layers do (services/api/internal/canon).
describe("canon arithmetic invariants", () => {
  it("all four equations hold against the fixtures", () => {
    expect(() => assertArithmetic()).not.toThrow();
  });

  it("ratified anchors are present (a zeroed fixture can't pass)", () => {
    expect(ecommerceProjectCents()).toBe(20800);
    expect(billing().resources_cents).toBe(38300);
    expect(billing().forecast_cents).toBe(48200);
  });

  it("the six ecommerce services sum to the $208 project total", () => {
    const sum = services().reduce((a, s) => a + s.monthly_estimate_cents, 0);
    expect(sum).toBe(ecommerceProjectCents());
    expect(services()).toHaveLength(6);
  });

  it("a broken fixture is rejected (the guard is not vacuous)", () => {
    const broken = structuredClone(fixturesJson) as Record<string, unknown>;
    const svcKey = Object.keys(broken).find((k) => k.startsWith("services"))!;
    (broken[svcKey] as { monthly_estimate_cents: number }[])[0].monthly_estimate_cents += 1;
    expect(() => assertArithmetic(broken)).toThrow(/Σ services/);
  });
});
