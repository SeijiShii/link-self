import { describe, expect, it } from "vitest";
import { generateIdentity, type Identity } from "../src/did.js";
import { createInvite } from "../src/invitation.js";
import {
  decodeJoinRequest,
  decodeJoinResponse,
  encodeJoinRequest,
  encodeJoinResponse,
  JoinService,
  MemConsumedNonceStore,
  type JoinRequest,
  type JoinResponse,
} from "../src/join.js";
import { MemNetworkStore, NetworkService } from "../src/network.js";
import { RoleDAG } from "../src/role.js";

const SUITE = "home-visit-suite";
const DAG = RoleDAG.build({
  member: { includes: [] },
  editor: { includes: ["member"] },
  admin: { includes: ["editor"] },
});
const RELAYS = ["/dns4/relay.example/tcp/443/wss/p2p/12D3KooWRelay"];

/** Build a network owned by `admin`, plus a JoinService accepting on its behalf. */
async function setup() {
  const store = new MemNetworkStore();
  const networks = new NetworkService(store, DAG, "admin");
  const admin = await generateIdentity();
  const networkId = await networks.create(SUITE, admin.did);
  const nonces = new MemConsumedNonceStore();
  const now = () => 1_000_000;
  const join = new JoinService(networks, store, DAG, "admin", nonces, SUITE, now);
  return { store, networks, admin, networkId, join, now };
}

async function invite(
  issuer: Identity,
  networkId: string,
  now: () => number,
  role = "member",
) {
  return createInvite(issuer, { networkId, suiteId: SUITE, role, relays: RELAYS }, 60_000, now);
}

describe("JoinService.accept", () => {
  it("adds the invitee and returns the updated network snapshot", async () => {
    const { admin, networkId, join, now } = await setup();
    const invitee = await generateIdentity();
    const req: JoinRequest = {
      v: 1,
      invite: await invite(admin, networkId, now),
      inviteeDID: invitee.did,
      displayName: "Newcomer",
    };
    const res = await join.accept(admin.did, req);
    expect(res.ok).toBe(true);
    if (!res.ok) return;
    expect(res.network.members).toContain(invitee.did);
    expect(res.network.memberRoles[invitee.did]).toBe("member");
    expect(res.network.networkId).toBe(networkId);
  });

  it("rejects an expired invite", async () => {
    const { admin, networkId, join } = await setup();
    const invitee = await generateIdentity();
    const past = () => 0;
    const req: JoinRequest = {
      v: 1,
      invite: await invite(admin, networkId, past), // expires at 60_000, now()=1_000_000
      inviteeDID: invitee.did,
      displayName: "x",
    };
    expect(await join.accept(admin.did, req)).toEqual({ ok: false, code: "invite_expired" });
  });

  it("rejects a tampered invite", async () => {
    const { admin, networkId, join, now } = await setup();
    const invitee = await generateIdentity();
    const good = await invite(admin, networkId, now);
    const req: JoinRequest = {
      v: 1,
      invite: { ...good, role: "admin" },
      inviteeDID: invitee.did,
      displayName: "x",
    };
    expect(await join.accept(admin.did, req)).toEqual({ ok: false, code: "invite_invalid" });
  });

  it("rejects an invite for a different suite", async () => {
    const { admin, networkId, join, now } = await setup();
    const invitee = await generateIdentity();
    const foreign = await createInvite(
      admin,
      { networkId, suiteId: "other-app", role: "member", relays: RELAYS },
      60_000,
      now,
    );
    const req: JoinRequest = { v: 1, invite: foreign, inviteeDID: invitee.did, displayName: "x" };
    expect(await join.accept(admin.did, req)).toEqual({ ok: false, code: "invite_invalid" });
  });

  it("rejects an invite for an unknown network", async () => {
    const { admin, join, now } = await setup();
    const invitee = await generateIdentity();
    const req: JoinRequest = {
      v: 1,
      invite: await invite(admin, "net-does-not-exist", now),
      inviteeDID: invitee.did,
      displayName: "x",
    };
    expect(await join.accept(admin.did, req)).toEqual({ ok: false, code: "network_not_found" });
  });

  it("rejects an invite signed by a non-admin member", async () => {
    const { admin, networkId, networks, join, now } = await setup();
    const editor = await generateIdentity();
    await networks.addMember(networkId, admin.did, editor.did, "editor"); // editor < admin
    const invitee = await generateIdentity();
    const req: JoinRequest = {
      v: 1,
      invite: await invite(editor, networkId, now),
      inviteeDID: invitee.did,
      displayName: "x",
    };
    expect(await join.accept(admin.did, req)).toEqual({ ok: false, code: "inviter_not_admin" });
  });

  it("rejects acceptance by a non-admin node", async () => {
    const { admin, networkId, networks, store, now } = await setup();
    const member = await generateIdentity();
    await networks.addMember(networkId, admin.did, member.did, "member");
    const nonAdminJoin = new JoinService(
      networks,
      store,
      DAG,
      "admin",
      new MemConsumedNonceStore(),
      SUITE,
      now,
    );
    const invitee = await generateIdentity();
    const req: JoinRequest = {
      v: 1,
      invite: await invite(admin, networkId, now),
      inviteeDID: invitee.did,
      displayName: "x",
    };
    expect(await nonAdminJoin.accept(member.did, req)).toEqual({ ok: false, code: "not_admin" });
  });

  it("rejects a re-used (consumed) invite", async () => {
    const { admin, networkId, join, now } = await setup();
    const inv = await invite(admin, networkId, now);
    const first: JoinRequest = {
      v: 1,
      invite: inv,
      inviteeDID: (await generateIdentity()).did,
      displayName: "a",
    };
    expect((await join.accept(admin.did, first)).ok).toBe(true);
    const second: JoinRequest = {
      v: 1,
      invite: inv,
      inviteeDID: (await generateIdentity()).did,
      displayName: "b",
    };
    expect(await join.accept(admin.did, second)).toEqual({ ok: false, code: "invite_consumed" });
  });

  it("rejects when the invitee is already a member", async () => {
    const { admin, networkId, networks, join, now } = await setup();
    const invitee = await generateIdentity();
    await networks.addMember(networkId, admin.did, invitee.did, "member");
    const req: JoinRequest = {
      v: 1,
      invite: await invite(admin, networkId, now),
      inviteeDID: invitee.did,
      displayName: "x",
    };
    expect(await join.accept(admin.did, req)).toEqual({ ok: false, code: "already_member" });
  });
});

describe("join wire codec", () => {
  it("round-trips a request", async () => {
    const { admin, networkId, now } = await setup();
    const req: JoinRequest = {
      v: 1,
      invite: await invite(admin, networkId, now),
      inviteeDID: (await generateIdentity()).did,
      displayName: "Newcomer",
    };
    expect(decodeJoinRequest(encodeJoinRequest(req))).toEqual(req);
  });

  it("round-trips success and failure responses", () => {
    const ok: JoinResponse = {
      ok: true,
      network: { networkId: "n", suiteId: SUITE, members: ["a", "b"], memberRoles: { a: "admin", b: "member" } },
    };
    const bad: JoinResponse = { ok: false, code: "invite_expired" };
    expect(decodeJoinResponse(encodeJoinResponse(ok))).toEqual(ok);
    expect(decodeJoinResponse(encodeJoinResponse(bad))).toEqual(bad);
  });
});
