import { describe, expect, it } from "vitest";
import { decide, initialsOf, sanitizeReturnTo } from "../src/lib/session";

describe("decide", () => {
  it("sends unauthenticated app traffic to /login", () => {
    expect(decide({ authed: false, routeKind: "app" })).toBe("/login");
    expect(decide({ authed: false, routeKind: "onboarding" })).toBe("/login");
  });

  it("bounces authed users off auth routes", () => {
    expect(decide({ authed: true, routeKind: "auth" })).toBe("/");
  });

  it("lets valid traffic through", () => {
    expect(decide({ authed: true, routeKind: "app" })).toBeNull();
    expect(decide({ authed: false, routeKind: "auth" })).toBeNull();
  });
});

describe("sanitizeReturnTo", () => {
  it("accepts same-origin relative paths", () => {
    expect(sanitizeReturnTo("/acme/ecommerce?env=staging")).toBe("/acme/ecommerce?env=staging");
  });

  it("rejects absolute/protocol-relative URLs", () => {
    expect(sanitizeReturnTo("https://evil.dev")).toBeNull();
    expect(sanitizeReturnTo("//evil.dev")).toBeNull();
  });

  it("never bounces back into auth or onboarding", () => {
    expect(sanitizeReturnTo("/login")).toBeNull();
    expect(sanitizeReturnTo("/signup")).toBeNull();
    expect(sanitizeReturnTo("/onboarding/org")).toBeNull();
  });
});

describe("initialsOf", () => {
  it("Priya Sharma → PS", () => {
    expect(initialsOf("Priya Sharma")).toBe("PS");
  });
  it("single names use one letter", () => {
    expect(initialsOf("asha")).toBe("A");
  });
});
