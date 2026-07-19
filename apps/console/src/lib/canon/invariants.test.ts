import { assertArithmetic, fixtures as canonFixtures } from "@steloit/canon";
import { describe, expect, it } from "vitest";
import consoleFixtures from "./fixtures.json";

// The console layer of the three-layer arithmetic invariant (Q2). The console
// renders costs from its own fixtures copy; this proves that copy satisfies the
// SAME equations the Go estimate/invoice layers and the @steloit/canon package
// assert — the check LOGIC is imported, never retyped.
describe("console canon arithmetic", () => {
  it("the console fixtures satisfy the canon invariants", () => {
    expect(() => assertArithmetic(consoleFixtures)).not.toThrow();
  });

  // Drift guard: the console copy must stay byte-identical to the canonical
  // fixtures (both synced by `make gen-ts`; CI's git diff also catches a stale
  // copy, this catches a hand-edit that skipped gen).
  it("the console fixtures copy matches the canonical source", () => {
    expect(consoleFixtures).toEqual(canonFixtures);
  });
});
