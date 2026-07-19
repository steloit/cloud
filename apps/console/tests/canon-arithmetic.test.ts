import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { assertArithmetic, sectionOf } from "@steloit/canon";
import { describe, expect, it } from "vitest";
import fixtures from "../src/lib/canon/fixtures.json";

/**
 * The console layer of the three-layer arithmetic invariant (Q2). The check
 * LOGIC is imported from @steloit/canon — the SAME assertArithmetic the Go
 * estimate/invoice layers mirror — and run over the console's own fixtures
 * copy. The four equations are no longer retyped here.
 */

const raw = fixtures as unknown as Record<string, unknown>;

// The one source of truth: docs/product/19-canon/fixtures.json, read directly
// so a console copy that drifted from canon (a canon edit that skipped `make
// gen-ts`) fails HERE, in the console job that runs in CI.
const canonicalSource = JSON.parse(
  readFileSync(
    fileURLToPath(new URL("../../../docs/product/19-canon/fixtures.json", import.meta.url)),
    "utf8",
  ),
);

describe("canon arithmetic invariants (16-qa)", () => {
  it("the four equations hold over the console fixtures", () => {
    expect(() => assertArithmetic(fixtures)).not.toThrow();
  });

  it("ratified anchors are present (a uniformly-corrupt copy can't pass)", () => {
    const bill = sectionOf<{ resources_cents: number; plan_fee_cents: number; forecast_cents: number }>(
      raw,
      "billing_overview",
    );
    expect(bill.resources_cents).toBe(38300);
    expect(bill.plan_fee_cents).toBe(9900);
    expect(bill.forecast_cents).toBe(48200);
  });

  it("billing by_project forecasts reconcile with project costs", () => {
    const billing = sectionOf<{ by_project: Array<{ name: string; forecast_cents: number }> }>(
      raw,
      "billing_overview",
    );
    const projects = sectionOf<Array<{ name: string; monthly_cost_cents?: number }>>(raw, "projects");
    for (const line of billing.by_project) {
      const project = projects.find((p) => p.name === line.name);
      expect(project?.monthly_cost_cents).toBe(line.forecast_cents);
    }
  });

  // Drift guard: the console copy must equal the CANONICAL source (not a
  // sibling copy that could be equally stale).
  it("the console fixtures copy matches the canonical source", () => {
    expect(fixtures).toEqual(canonicalSource);
  });
});
