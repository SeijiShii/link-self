/**
 * End-to-end test of multi-device sync: two LinkSelfClients that share ONE DID
 * but run on separate libp2p nodes with DISTINCT transport keys (hence distinct
 * peer IDs) — the browser multi-device model (wants/01: "same DID full copy").
 * They mutual-auth (peerId decoupled from DID), and a MyDB KV write on one
 * device replicates to the other via devicesync.
 */
import { noise } from "@chainsafe/libp2p-noise";
import { yamux } from "@chainsafe/libp2p-yamux";
import { generateKeyPair } from "@libp2p/crypto/keys";
import { webSockets } from "@libp2p/websockets";
import { multiaddr } from "@multiformats/multiaddr";
import { createLibp2p, type Libp2p } from "libp2p";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { LinkSelfClient } from "../src/client.js";
import { generateIdentity, type Identity } from "../src/did.js";

const enc = new TextEncoder();
const dec = new TextDecoder();

/** A device: its own transport key (peerId), but a shared user identity (DID). */
async function makeDevice(identity: Identity): Promise<{
  libp2p: Libp2p;
  client: LinkSelfClient;
}> {
  // Transport key is independent of the DID key — this is the decoupling.
  const transportKey = await generateKeyPair("Ed25519");
  const libp2p = await createLibp2p({
    privateKey: transportKey,
    addresses: { listen: ["/ip4/127.0.0.1/tcp/0/ws"] },
    transports: [webSockets()],
    connectionEncrypters: [noise()],
    streamMuxers: [yamux()],
  });
  const client = new LinkSelfClient({ libp2p, identity });
  await client.start();
  return { libp2p, client };
}

async function eventually<T>(
  fn: () => Promise<T | null | false> | (T | null | false),
  timeoutMs = 10_000,
): Promise<T> {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    const v = await fn();
    if (v != null && v !== false) return v;
    if (Date.now() > deadline) throw new Error("condition not met in time");
    await new Promise((r) => setTimeout(r, 50));
  }
}

describe("devicesync between two devices sharing one DID", () => {
  let identity: Identity;
  let dev1: Awaited<ReturnType<typeof makeDevice>>;
  let dev2: Awaited<ReturnType<typeof makeDevice>>;

  beforeAll(async () => {
    identity = await generateIdentity();
    dev1 = await makeDevice(identity);
    dev2 = await makeDevice(identity);
  }, 60_000);

  afterAll(async () => {
    await dev1?.libp2p.stop();
    await dev2?.libp2p.stop();
  });

  it("the two devices have the same DID but different transport peer IDs", () => {
    expect(dev1.client.did).toBe(dev2.client.did);
    expect(dev1.libp2p.peerId.toString()).not.toBe(
      dev2.libp2p.peerId.toString(),
    );
  });

  it("mutual-auth registers each device as a peer of the shared DID", async () => {
    const d2Addr = dev2.libp2p
      .getMultiaddrs()
      .find((m) => m.toString().includes("/ws"))!;
    await dev1.client.node.connectToAddr(
      identity.did,
      multiaddr(d2Addr.toString()),
      { mutual: true },
    );

    // Initiator (dev1) records dev2 immediately; responder (dev2) records
    // dev1 once its handler finishes — hence the poll.
    expect(dev1.client.node.peersForDID(identity.did)).toContain(
      dev2.libp2p.peerId.toString(),
    );
    await eventually(() =>
      dev2.client.node
        .peersForDID(identity.did)
        .includes(dev1.libp2p.peerId.toString()),
    );
  }, 30_000);

  it("replicates a KV write from dev1 to dev2", async () => {
    await dev1.client.myDB.put("my_settings", "locale", enc.encode("ja"));
    const rec = await eventually(async () => {
      const r = await dev2.client.myDB.get("my_settings", "locale");
      return r?.body != null ? r : null;
    });
    expect(dec.decode(rec.body!)).toBe("ja");
  }, 30_000);

  it("replicates a KV write in the reverse direction (dev2 → dev1)", async () => {
    await dev2.client.myDB.put("my_settings", "aiModel", enc.encode("opus"));
    const rec = await eventually(async () => {
      const r = await dev1.client.myDB.get("my_settings", "aiModel");
      return r?.body != null ? r : null;
    });
    expect(dec.decode(rec.body!)).toBe("opus");
  }, 30_000);
});
