/**
 * FastStart / peerstore persistence (browser counterpart of
 * mobile-support §3.1.3 / §3.1.4):
 *   1. snapshotKnownPeers → new client instance (same identity, fresh
 *      libp2p) auto-reconnects on start() from the hints alone
 *   2. a persistent libp2p datastore carries peer addresses across
 *      libp2p instances (what IndexedDB provides in the browser)
 */
import { noise } from "@chainsafe/libp2p-noise";
import { yamux } from "@chainsafe/libp2p-yamux";
import { webSockets } from "@libp2p/websockets";
import { MemoryDatastore } from "datastore-core";
import { createLibp2p, type Libp2p } from "libp2p";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { LinkSelfClient, type KnownPeer } from "../src/client.js";
import { didToPeerId, generateIdentity, type Identity } from "../src/did.js";

const enc = new TextEncoder();
const dec = new TextDecoder();

async function makeLibp2p(
  identity: Identity,
  opts: { listen?: boolean; datastore?: MemoryDatastore } = {},
): Promise<Libp2p> {
  return await createLibp2p({
    privateKey: identity.privateKey,
    ...(opts.listen === true
      ? { addresses: { listen: ["/ip4/127.0.0.1/tcp/0/ws"] } }
      : {}),
    ...(opts.datastore != null ? { datastore: opts.datastore } : {}),
    transports: [webSockets()],
    connectionEncrypters: [noise()],
    streamMuxers: [yamux()],
  });
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

describe("FastStart (known-peer hints)", () => {
  let serverLibp2p: Libp2p;
  let serverIdentity: Identity;
  let serverClient: LinkSelfClient;
  const stopped: Libp2p[] = [];

  beforeAll(async () => {
    serverIdentity = await generateIdentity();
    serverLibp2p = await makeLibp2p(serverIdentity, { listen: true });
    serverClient = new LinkSelfClient({
      libp2p: serverLibp2p,
      identity: serverIdentity,
    });
    await serverClient.start();
  });

  afterAll(async () => {
    await serverLibp2p.stop();
    for (const l of stopped) {
      try {
        await l.stop();
      } catch {
        // already stopped
      }
    }
  });

  it("snapshot → restart → auto-reconnect without explicit dialing", async () => {
    // Session 1: connect explicitly, snapshot the known peers.
    const leafIdentity = await generateIdentity();
    const libp2p1 = await makeLibp2p(leafIdentity);
    stopped.push(libp2p1);
    const client1 = new LinkSelfClient({
      libp2p: libp2p1,
      identity: leafIdentity,
    });
    await client1.start();
    const serverAddr = serverLibp2p
      .getMultiaddrs()
      .find((m) => m.toString().includes("/ws"))!;
    await client1.node.connectToAddr(serverIdentity.did, serverAddr);

    const snapshot: KnownPeer[] = client1.snapshotKnownPeers();
    expect(snapshot).toHaveLength(1);
    expect(snapshot[0]!.did).toBe(serverIdentity.did);
    expect(snapshot[0]!.addrs[0]).toContain("/ws");
    expect(snapshot[0]!.addrs[0]).toContain("/p2p/");

    // "App shutdown": stop the first session entirely.
    await libp2p1.stop();

    // Session 2: same identity, fresh libp2p — FastStart from the snapshot.
    const libp2p2 = await makeLibp2p(leafIdentity);
    stopped.push(libp2p2);
    const client2 = new LinkSelfClient({
      libp2p: libp2p2,
      identity: leafIdentity,
      knownPeers: snapshot,
    });

    const received: string[] = [];
    serverClient.node.setOnMessage((_did, payload) => {
      received.push(dec.decode(payload));
    });

    await client2.start(); // dials + authenticates from hints alone

    const serverPeerId = didToPeerId(serverIdentity.did);
    expect(libp2p2.getConnections(serverPeerId).length).toBeGreaterThan(0);

    // The revived connection is immediately usable.
    await client2.node.sendMessage(
      serverIdentity.did,
      enc.encode("after faststart"),
    );
    await eventually(async () => received.includes("after faststart"));
  }, 30_000);

  it("unreachable hints are skipped without failing start", async () => {
    const identity = await generateIdentity();
    const libp2p = await makeLibp2p(identity);
    stopped.push(libp2p);
    const client = new LinkSelfClient({
      libp2p,
      identity,
      knownPeers: [
        {
          did: serverIdentity.did,
          addrs: [
            "/ip4/127.0.0.1/tcp/1/ws/p2p/" +
              didToPeerId(serverIdentity.did).toString(),
          ],
        },
      ],
    });
    await client.start(); // no throw
    expect(libp2p.getConnections()).toHaveLength(0);
  });
});

describe("peerstore persistence via a libp2p datastore", () => {
  it("peer addresses survive across libp2p instances sharing a datastore", async () => {
    const serverIdentity = await generateIdentity();
    const server = await makeLibp2p(serverIdentity, { listen: true });
    const serverAddr = server
      .getMultiaddrs()
      .find((m) => m.toString().includes("/ws"))!;

    const identity = await generateIdentity();
    const datastore = new MemoryDatastore(); // browser: datastore-idb (IndexedDB)

    const session1 = await makeLibp2p(identity, { datastore });
    await session1.dial(serverAddr);
    const serverPeerId = didToPeerId(serverIdentity.did);
    await eventually(
      async () =>
        (await session1.peerStore.get(serverPeerId).catch(() => null))
          ?.addresses.length ?? null,
    );
    await session1.stop();

    // New libp2p instance, same datastore — the peerstore is warm.
    const session2 = await makeLibp2p(identity, { datastore });
    const restored = await session2.peerStore.get(serverPeerId);
    expect(restored.addresses.length).toBeGreaterThan(0);

    // Warm peerstore = dialable by peer id alone, no multiaddr needed.
    const conn = await session2.dial(serverPeerId);
    expect(conn.remotePeer.equals(serverPeerId)).toBe(true);

    await session2.stop();
    await server.stop();
  }, 30_000);
});
