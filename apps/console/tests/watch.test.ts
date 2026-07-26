import { describe, expect, it } from "vitest";
import { useWatchStore, watchProvisioning } from "../src/lib/watch";

/**
 * T8.2 / U5 — the registry is MODULE-scoped state, structurally independent of
 * any drawer's lifecycle: a drawer registers an id and may unmount; the shell's
 * ProvisioningWatcher (the only poller) reads this store. These tests pin the
 * registry semantics the survival guarantee rests on.
 */
describe("provisioning watch registry (U5: survives drawer close)", () => {
  it("registers, dedups, and unregisters", () => {
    useWatchStore.setState({ watched: [] });
    watchProvisioning("svc_a");
    watchProvisioning("svc_a"); // dedup
    watchProvisioning("svc_b");
    expect(useWatchStore.getState().watched).toEqual(["svc_a", "svc_b"]);
    useWatchStore.getState().unwatch("svc_a");
    expect(useWatchStore.getState().watched).toEqual(["svc_b"]);
  });

  it("holds state with no component mounted (the survival property)", () => {
    useWatchStore.setState({ watched: [] });
    // a 'drawer' registers then disappears — nothing here is tied to React
    watchProvisioning("svc_survives");
    // …time passes, no component exists…
    expect(useWatchStore.getState().watched).toContain("svc_survives");
  });
});
