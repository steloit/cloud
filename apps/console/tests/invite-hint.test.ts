import { describe, expect, it } from "vitest";
import { matchesHint } from "../src/features/invites/hint";

describe("matchesHint (A6/A7 wrong-account detection)", () => {
  it("marco@acme.dev matches m•••o@acme.dev", () => {
    expect(matchesHint("marco@acme.dev", "m•••o@acme.dev")).toBe(true);
  });

  it("priya@acme.dev does not match m•••o@acme.dev", () => {
    expect(matchesHint("priya@acme.dev", "m•••o@acme.dev")).toBe(false);
  });

  it("sam@lumon.dev does not match m•••o@acme.dev (domain differs)", () => {
    expect(matchesHint("sam@lumon.dev", "m•••o@acme.dev")).toBe(false);
  });

  it("malformed inputs never match", () => {
    expect(matchesHint("not-an-email", "m•••o@acme.dev")).toBe(false);
    expect(matchesHint("marco@acme.dev", "")).toBe(false);
  });
});
