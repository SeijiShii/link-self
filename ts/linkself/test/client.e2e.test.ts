/**
 * End-to-end test of the assembled client: two LinkSelfClients on real
 * libp2p nodes (WebSocket over localhost) exchange group-shared records
 * with subscription filtering — auth, store-and-forward flush of the
 * subscription announcement, envelope routing, and LWW apply, all live.
 */
import { noise } from "@chainsafe/libp2p-noise";
import { yamux } from "@chainsafe/libp2p-yamux";
import { webSockets } from "@libp2p/websockets";
import { multiaddr } from "@multiformats/multiaddr";
import { createLibp2p, type Libp2p } from "libp2p";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { DeviceSyncSubscriptionStore, LinkSelfClient } from "../src/client.js";
import { generateIdentity, type Identity } from "../src/did.js";
import { MemDeviceStorage, ReplicationEngine } from "../src/devicesync.js";
import type { Network, NetworkStore } from "../src/network.js";

const enc = new TextEncoder();
const dec = new TextDecoder();

/** A NetworkStore that serves one fixed network (test fixture). */
class FixedNetworkStore implements NetworkStore {
  constructor(private readonly network: Network) {}
  async createNetwork(): Promise<string> {
    return this.network.id;
  }
  async getNetwork(id: string): Promise<Network | null> {
    return id === this.network.id ? this.network : null;
  }
  async updateNetwork(): Promise<void> {}
  async deleteNetwork(): Promise<void> {}
  async putNetwork(): Promise<void> {}
  async listForMember(memberDID: string): Promise<string[]> {
    return this.network.members.includes(memberDID) ? [this.network.id] : [];
  }
}

async function makePeer(network: Network): Promise<{
  libp2p: Libp2p;
  identity: Identity;
  client: LinkSelfClient;
}> {
  const identity = await generateIdentity();
  const libp2p = await createLibp2p({
    privateKey: identity.privateKey,
    addresses: { listen: ["/ip4/127.0.0.1/tcp/0/ws"] },
    transports: [webSockets()],
    connectionEncrypters: [noise()],
    streamMuxers: [yamux()],
  });
  const client = new LinkSelfClient({
    libp2p,
    identity,
    roles: { admin: { includes: ["editor"] }, editor: { includes: [] } },
    networkStore: new FixedNetworkStore(network),
  });
  await client.start();
  client.groupShare.registerChannel({
    name: "visits",
    groupId: network.id,
    perms: { read: "editor", write: "editor", delete: "editor" },
  });
  return { libp2p, identity, client };
}

async function eventually<T>(
  fn: () => Promise<T | null | false>,
  timeoutMs = 10_000,
): Promise<T> {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    const v = await fn();
    if (v != null && v !== false) return v;
    if (Date.now() > deadline) throw new Error("condition not met in time");
    await new Promise((r) => setTimeout(r, 100));
  }
}

describe("LinkSelfClient e2e (two real libp2p nodes)", () => {
  let a: Awaited<ReturnType<typeof makePeer>>;
  let b: Awaited<ReturnType<typeof makePeer>>;

  beforeAll(async () => {
    // The network fixture is completed with real DIDs after key generation,
    // so build peers first with a shared mutable network object.
    const network: Network = {
      id: "net-1",
      suiteId: "jp.test",
      members: [],
      memberRoles: {},
    };
    a = await makePeer(network);
    b = await makePeer(network);
    network.members = [a.identity.did, b.identity.did];
    network.memberRoles = {
      [a.identity.did]: "editor",
      [b.identity.did]: "editor",
    };
  }, 60_000);

  afterAll(async () => {
    await a?.libp2p.stop();
    await b?.libp2p.stop();
  });

  it("subscription announced before connect is flushed on auth and filters broadcasts", async () => {
    // B subscribes while offline — the announcement is queued (store-and-forward).
    await b.client.groupShare.subscribe("visits", ["area/1"]);
    expect(
      b.client.node.storeForward.pendingCount(a.identity.did),
    ).toBeGreaterThan(0);

    // B connects to A; auth flushes the queued announcement.
    const aAddr = a.libp2p
      .getMultiaddrs()
      .find((m) => m.toString().includes("/ws"))!;
    await b.client.node.connectToAddr(
      a.identity.did,
      multiaddr(aAddr.toString()),
    );

    // A eventually records B's subscription.
    const subs = await eventually(
      async () =>
        await a.client.remoteSubs.getSubscription(b.identity.did, "visits"),
    );
    expect(subs).toEqual(["area/1"]);

    // A puts a record with a matching topic → B receives and applies it.
    await a.client.groupShare.put(
      "visits",
      "area/1",
      "rec-1",
      enc.encode('{"v":1}'),
    );
    const got = await eventually(
      async () => await b.client.groupShare.get("visits", "rec-1"),
    );
    expect(dec.decode(got.body!)).toBe('{"v":1}');
    expect(got.did).toBe(a.identity.did);

    // A puts a record with a non-matching topic → B is filtered out.
    await a.client.groupShare.put(
      "visits",
      "area/9",
      "rec-2",
      enc.encode('{"v":2}'),
    );
    await new Promise((r) => setTimeout(r, 800));
    expect(await b.client.groupShare.get("visits", "rec-2")).toBeNull();
  }, 30_000);

  it("plain messages round-trip through the assembled clients", async () => {
    const received: string[] = [];
    a.client.node.setOnMessage((_did, payload) => {
      received.push(dec.decode(payload));
    });
    await b.client.sendToGroup(
      [a.identity.did, b.identity.did],
      enc.encode("hello group"),
    );
    await eventually(async () => received.includes("hello group"));
  }, 30_000);
});

describe("DeviceSyncSubscriptionStore", () => {
  it("stores and lists subscriptions via the replication engine", async () => {
    const engine = new ReplicationEngine({
      storage: new MemDeviceStorage(),
      selfDID: "did:self",
    });
    const store = new DeviceSyncSubscriptionStore(engine);
    expect(await store.getSubscription("did:self", "visits")).toBeNull();
    await store.setSubscription("did:self", "visits", ["a", "b"]);
    await store.setSubscription("did:self", "places", ["*"]);
    expect(await store.getSubscription("did:self", "visits")).toEqual([
      "a",
      "b",
    ]);
    const all = await store.getAllSubscriptions("did:self");
    expect(all.get("visits")).toEqual(["a", "b"]);
    expect(all.get("places")).toEqual(["*"]);
  });
});
