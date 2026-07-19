import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import fixturesJson from "./fixtures.json" with { type: "json" };
import { assertArithmetic, billing, ecommerceProjectCents, services } from "./index.js";

// The TS layer of the three-layer arithmetic invariant (Q2). The console
// imports assertArithmetic and runs it over its own fixtures copy; here we run
// it over the canonical package copy. Both assert the same equations the Go
// estimate/invoice layers do (services/api/internal/canon).

// The one source of truth: docs/product/19-canon/fixtures.json. Read directly
// (not via the synced copy) so this test catches a stale copy in-process —
// independent of whether CI ran `make gen-ts`.
const canonicalSource = JSON.parse(
  readFileSync(fileURLToPath(new URL("../../../docs/product/19-canon/fixtures.json", import.meta.url)), "utf8"),
);

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

  // The real drift catch: the synced copy must equal the canonical source. A
  // canon edit committed without `make gen-ts` fails HERE, in-process.
  it("the package fixtures copy matches the canonical source", () => {
    expect(fixturesJson).toEqual(canonicalSource);
  });

  it("a broken fixture is rejected (the guard is not vacuous)", () => {
    const broken = structuredClone(fixturesJson) as Record<string, unknown>;
    const svcKey = Object.keys(broken).find((k) => k.startsWith("services"))!;
    (broken[svcKey] as { monthly_estimate_cents: number }[])[0].monthly_estimate_cents += 1;
    expect(() => assertArithmetic(broken)).toThrow(/Σ services/);
  });

  it("a zeroed world is rejected, not passed with 0 === 0", () => {
    const zeroed = structuredClone(fixturesJson) as Record<string, unknown>;
    const projKey = Object.keys(zeroed).find((k) => k.startsWith("projects"))!;
    for (const p of zeroed[projKey] as { monthly_cost_cents: number }[]) p.monthly_cost_cents = 0;
    expect(() => assertArithmetic(zeroed)).toThrow(/missing or zero/);
  });
});
