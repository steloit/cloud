import { describe, expect, it } from "vitest";
import { fmtMoney, fmtMoneyPerMonth } from "../src/lib/fmt";

describe("fmtMoney (ADR-025: integer cents in, mono-rendered dollars out)", () => {
  it("whole dollars drop the cents: 20800 → $208", () => {
    expect(fmtMoney(20800)).toBe("$208");
    expect(fmtMoney(48200)).toBe("$482");
    expect(fmtMoney(0)).toBe("$0");
  });

  it("fractional amounts keep two places: 19910 → $199.10", () => {
    expect(fmtMoney(19910)).toBe("$199.10");
    expect(fmtMoney(17142)).toBe("$171.42");
    expect(fmtMoney(670)).toBe("$6.70");
    expect(fmtMoney(220)).toBe("$2.20");
  });

  it("thousands are grouped", () => {
    expect(fmtMoney(123456700)).toBe("$1,234,567");
  });

  it("per-month grammar", () => {
    expect(fmtMoneyPerMonth(5800)).toBe("$58/mo");
  });
});
