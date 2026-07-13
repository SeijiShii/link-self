/**
 * End-to-end test of membership propagation: when one admin accepts a join,
 * the other admin converges on the new roster via a `network_meta` broadcast,
 * and the invitee bootstraps the network locally from the join response.
 */
import { noise } from "@chainsafe/libp2p-noise";
import { yamux } from "@chainsafe/libp2p-yamux";
import { webSockets } from "@libp2p/websockets";
import { multiaddr } from "@multiformats/multiaddr";
import { createLibp2p, type Libp2p } from "libp2p";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { LinkSelfClient } from "../src/client.js";
import { generateIdentity, type Identity } from "../src/did.js";
import { createInvite } from "../src/invitation.js";
import { MemNetworkStore, type Network } from "../src/network.js";

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

async function eventually<T>(fn: () => Promise<T | null | false | undefined>, timeoutMs = 10_000): Promise<T> {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    const v = await fn();
    if (v != null && v !== false) return v;
    if (Date.now() > deadline) throw new Error("condition not met in time");
    await new Promise((r) => setTimeout(r, 100));
  }
}

describe("membership propagation e2e", () => {
  let admin1: Peer;
  let admin2: Peer;
  let invitee: Peer;
  let networkId: string;

  beforeAll(async () => {
    [admin1, admin2, invitee] = await Promise.all([makePeer(), makePeer(), makePeer()]);

    // admin1 owns the network and appoints admin2 as a second admin.
    networkId = await admin1.client.network.create(SUITE, admin1.identity.did);
    await admin1.client.network.addMember(
      networkId,
      admin1.identity.did,
      admin2.identity.did,
      "admin",
    );

    // Seed admin2's local view (as if it had bootstrapped earlier), so it knows
    // admin1 is an admin and will accept its membership broadcasts.
    const seed: Network = {
      id: networkId,
      suiteId: SUITE,
      members: [admin1.identity.did, admin2.identity.did],
      memberRoles: {
        [admin1.identity.did]: "admin",
        [admin2.identity.did]: "admin",
      },
    };
    await admin2.client.networkStore.putNetwork(seed);

    // admin2 connects to admin1 so broadcasts can reach it.
    await admin2.client.node.connectToAddr(admin1.identity.did, multiaddr(wsAddrOf(admin1)));
  }, 60_000);

  afterAll(async () => {
    await Promise.all([admin1, admin2, invitee].map((p) => p?.libp2p.stop()));
  });

  it("converges the second admin after the first accepts a join", async () => {
    const invite = await createInvite(
      admin1.identity,
      { networkId, suiteId: SUITE, role: "member", relays: [wsAddrOf(admin1)] },
      60_000,
    );

    const res = await invitee.client.requestJoin(wsAddrOf(admin1), invite, "Newcomer");
    expect(res.ok).toBe(true);

    // The invitee bootstrapped the network locally from the join response.
    const inviteeNet = await invitee.client.networkStore.getNetwork(networkId);
    expect(inviteeNet?.members).toContain(invitee.identity.did);

    // The second admin converges on the new member via network_meta.
    const converged = await eventually(async () => {
      const n = await admin2.client.networkStore.getNetwork(networkId);
      return n?.members.includes(invitee.identity.did) ? n : null;
    });
    expect(converged.memberRoles[invitee.identity.did]).toBe("member");
  }, 30_000);
});
