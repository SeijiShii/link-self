/**
 * Two-layer devicesync: two devices share one USER identity but each has its
 * own DEVICE identity (its libp2p key, peerId ≡ deviceDID). A user-key-signed
 * roster lists both device DIDs; devicesync targets and accepts only roster
 * members. Device-to-device auth is the plain one-way auth (peerId ≡ deviceDID)
 * plus the roster membership check — no mutual auth needed.
 */
import { noise } from "@chainsafe/libp2p-noise";
import { yamux } from "@chainsafe/libp2p-yamux";
import { webSockets } from "@libp2p/websockets";
import { multiaddr } from "@multiformats/multiaddr";
import { createLibp2p, type Libp2p } from "libp2p";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { LinkSelfClient } from "../src/client.js";
import { generateIdentity, type Identity } from "../src/did.js";
import { buildRoster } from "../src/roster.js";

const enc = new TextEncoder();
const dec = new TextDecoder();

async function makeDevice(userIdentity: Identity): Promise<{
  libp2p: Libp2p;
  client: LinkSelfClient;
  deviceDID: string;
}> {
  const deviceIdentity = await generateIdentity(); // device key = libp2p key
  const libp2p = await createLibp2p({
    privateKey: deviceIdentity.privateKey,
    addresses: { listen: ["/ip4/127.0.0.1/tcp/0/ws"] },
    transports: [webSockets()],
    connectionEncrypters: [noise()],
    streamMuxers: [yamux()],
  });
  const client = new LinkSelfClient({
    libp2p,
    identity: deviceIdentity, // transport/device identity
    userIdentity, // account identity (shared)
  });
  await client.start();
  return { libp2p, client, deviceDID: deviceIdentity.did };
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
    await new Promise((r) => setTimeout(r, 50));
  }
}

describe("two-layer roster devicesync", () => {
  let user: Identity;
  let dev1: Awaited<ReturnType<typeof makeDevice>>;
  let dev2: Awaited<ReturnType<typeof makeDevice>>;

  beforeAll(async () => {
    user = await generateIdentity();
    dev1 = await makeDevice(user);
    dev2 = await makeDevice(user);

    const roster = await buildRoster(user, [
      { deviceDID: dev1.deviceDID, label: "PC" },
      { deviceDID: dev2.deviceDID, label: "Phone" },
    ]);
    dev1.client.setRoster(roster);
    dev2.client.setRoster(roster);

    // dev1 dials dev2 by its DEVICE DID (peerId ≡ deviceDID ⇒ plain auth works).
    const d2Addr = dev2.libp2p
      .getMultiaddrs()
      .find((m) => m.toString().includes("/ws"))!;
    await dev1.client.node.connectToAddr(
      dev2.deviceDID,
      multiaddr(d2Addr.toString()),
    );
  }, 60_000);

  afterAll(async () => {
    await dev1?.libp2p.stop();
    await dev2?.libp2p.stop();
  });

  it("exposes the account DID as the client DID, not the device DID", () => {
    expect(dev1.client.did).toBe(user.did);
    expect(dev2.client.did).toBe(user.did);
    expect(dev1.deviceDID).not.toBe(user.did);
    expect(dev1.deviceDID).not.toBe(dev2.deviceDID);
  });

  it("replicates a KV write between roster siblings (dev1 → dev2)", async () => {
    await dev1.client.myDB.put("my_settings", "locale", enc.encode("ja"));
    const rec = await eventually(async () => {
      const r = await dev2.client.myDB.get("my_settings", "locale");
      return r?.body != null ? r : null;
    });
    expect(dec.decode(rec.body!)).toBe("ja");
  }, 30_000);

  it("replicates in reverse (dev2 → dev1)", async () => {
    await dev2.client.myDB.put("my_settings", "aiModel", enc.encode("opus"));
    const rec = await eventually(async () => {
      const r = await dev1.client.myDB.get("my_settings", "aiModel");
      return r?.body != null ? r : null;
    });
    expect(dec.decode(rec.body!)).toBe("opus");
  }, 30_000);

  it("rejects devicesync from a device not in the receiver's roster", async () => {
    // Narrow dev2's roster to exclude dev1 ⇒ dev2 must drop dev1's writes.
    const soloRoster = await buildRoster(user, [
      { deviceDID: dev2.deviceDID, label: "Phone" },
    ]);
    dev2.client.setRoster(soloRoster);

    await dev1.client.myDB.put("my_settings", "secret", enc.encode("x"));
    await new Promise((r) => setTimeout(r, 600));
    expect(await dev2.client.myDB.get("my_settings", "secret")).toBeNull();

    // restore for isolation
    const full = await buildRoster(user, [
      { deviceDID: dev1.deviceDID, label: "PC" },
      { deviceDID: dev2.deviceDID, label: "Phone" },
    ]);
    dev2.client.setRoster(full);
  }, 30_000);
});
