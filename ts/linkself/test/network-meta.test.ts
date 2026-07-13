import { describe, expect, it } from "vitest";
import { MemNetworkStore, type Network } from "../src/network.js";
import {
  marshalNetworkMeta,
  metaOf,
  NetworkMetaTracker,
  unmarshalNetworkMeta,
  type NetworkMeta,
} from "../src/network-meta.js";

const NET: Network = {
  id: "net-1",
  suiteId: "jp.test",
  members: ["did:a", "did:b"],
  memberRoles: { "did:a": "admin", "did:b": "member" },
};

describe("network meta codec", () => {
  it("round-trips a snapshot", () => {
    const meta = metaOf(NET, 42);
    expect(unmarshalNetworkMeta(marshalNetworkMeta(meta))).toEqual(meta);
  });

  it("rejects malformed bytes", () => {
    expect(() => unmarshalNetworkMeta(new TextEncoder().encode("{}"))).toThrow();
  });
});

describe("NetworkMetaTracker (last-write-wins by epoch)", () => {
  it("applies a snapshot to the store and records the epoch", async () => {
    const store = new MemNetworkStore();
    const tracker = new NetworkMetaTracker(store);

    expect(await tracker.apply(metaOf(NET, 10))).toBe(true);
    expect((await store.getNetwork("net-1"))?.members).toEqual(["did:a", "did:b"]);
    expect(tracker.lastEpoch("net-1")).toBe(10);
  });

  it("applies a newer snapshot and ignores an older one", async () => {
    const store = new MemNetworkStore();
    const tracker = new NetworkMetaTracker(store);
    await tracker.apply(metaOf(NET, 10));

    const newer: NetworkMeta = {
      ...metaOf(NET, 20),
      members: ["did:a", "did:b", "did:c"],
      memberRoles: { "did:a": "admin", "did:b": "member", "did:c": "member" },
    };
    expect(await tracker.apply(newer)).toBe(true);
    expect((await store.getNetwork("net-1"))?.members).toContain("did:c");

    const older: NetworkMeta = { ...metaOf(NET, 15), members: ["did:a"] };
    expect(await tracker.apply(older)).toBe(false);
    expect((await store.getNetwork("net-1"))?.members).toContain("did:c");
  });

  it("noteEpoch blocks a subsequently received older snapshot", async () => {
    const store = new MemNetworkStore();
    const tracker = new NetworkMetaTracker(store);
    // We published our own change at epoch 30 (already applied locally).
    tracker.noteEpoch("net-1", 30);
    // A concurrently-published older snapshot must not regress us.
    expect(await tracker.apply(metaOf(NET, 25))).toBe(false);
    expect(await store.getNetwork("net-1")).toBeNull(); // nothing applied
  });
});
