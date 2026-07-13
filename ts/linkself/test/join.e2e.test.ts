/**
 * End-to-end test of the join handshake: LinkSelfClients on real libp2p nodes
 * (WebSocket over localhost). An admin owns a network; strangers dial the admin,
 * present an invitation, and are added live over `/linkself/join/1.0.0`.
 */
import { noise } from "@chainsafe/libp2p-noise";
import { yamux } from "@chainsafe/libp2p-yamux";
import { webSockets } from "@libp2p/websockets";
import { createLibp2p, type Libp2p } from "libp2p";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { LinkSelfClient } from "../src/client.js";
import { generateIdentity, type Identity } from "../src/did.js";
import { createInvite, type Invite } from "../src/invitation.js";
import { MemNetworkStore } from "../src/network.js";

const SUITE = "jp.test.suite";
const ROLES = { admin: { includes: ["member"] }, member: { includes: [] } };

type Peer = { libp2p: Libp2p; identity: Identity; client: LinkSelfClient };

async function makePeer(): Promise<Peer> {
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
    roles: ROLES,
    networkStore: new MemNetworkStore(),
  });
  await client.start();
  return { libp2p, identity, client };
}

function wsAddrOf(peer: Peer): string {
  return peer.libp2p
    .getMultiaddrs()
    .find((m) => m.toString().includes("/ws"))!
    .toString();
}

describe("join handshake e2e (two real libp2p nodes)", () => {
  let admin: Peer;
  let p1: Peer;
  let p2: Peer;
  let p3: Peer;
  let networkId: string;

  const inviteFrom = async (issuer: Peer, role = "member", ttlMs = 60_000): Promise<Invite> =>
    createInvite(
      issuer.identity,
      { networkId, suiteId: SUITE, role, relays: [wsAddrOf(admin)] },
      ttlMs,
    );

  beforeAll(async () => {
    [admin, p1, p2, p3] = await Promise.all([makePeer(), makePeer(), makePeer(), makePeer()]);
    networkId = await admin.client.network.create(SUITE, admin.identity.did);
  }, 60_000);

  afterAll(async () => {
    await Promise.all([admin, p1, p2, p3].map((p) => p?.libp2p.stop()));
  });

  it("adds an invitee who presents a valid 3-day invitation", async () => {
    const invite = await inviteFrom(admin, "member", 3 * 24 * 60 * 60_000);
    const res = await p1.client.requestJoin(wsAddrOf(admin), invite, "Newcomer");

    expect(res.ok).toBe(true);
    if (!res.ok) return;
    expect(res.network.members).toContain(p1.identity.did);
    expect(res.network.memberRoles[p1.identity.did]).toBe("member");

    // The admin's local network state now includes the invitee.
    const net = await admin.client.networkStore.getNetwork(networkId);
    expect(net?.members).toContain(p1.identity.did);
  }, 30_000);

  it("rejects a re-used invitation (single-use nonce)", async () => {
    const invite = await inviteFrom(admin);
    const first = await p2.client.requestJoin(wsAddrOf(admin), invite, "A");
    expect(first.ok).toBe(true);

    const second = await p3.client.requestJoin(wsAddrOf(admin), invite, "B");
    expect(second).toEqual({ ok: false, code: "invite_consumed" });
  }, 30_000);

  it("rejects an invitation forged by a non-admin", async () => {
    const forged = await inviteFrom(p3, "admin"); // p3 is not an admin member
    const res = await p3.client.requestJoin(wsAddrOf(admin), forged, "X");
    expect(res).toEqual({ ok: false, code: "inviter_not_admin" });
  }, 30_000);
});
