import { describe, expect, it } from "vitest";
import { generateIdentity } from "../src/did.js";
import {
  buildInviteUrl,
  createInvite,
  decodeInvite,
  encodeInvite,
  extractInviteParam,
  InviteError,
  verifyInvite,
} from "../src/invitation.js";

const PARAMS = {
  networkId: "net-1",
  suiteId: "home-visit-suite",
  role: "member",
  relays: ["/dns4/relay.example/tcp/443/wss/p2p/12D3KooWRelay"],
};

describe("createInvite", () => {
  it("mints a signed, time-limited invite carrying the claims (no key material)", async () => {
    const admin = await generateIdentity();
    const now = 1_000_000;
    const invite = await createInvite(admin, PARAMS, 60_000, () => now);

    expect(invite.v).toBe(1);
    expect(invite.networkId).toBe("net-1");
    expect(invite.suiteId).toBe("home-visit-suite");
    expect(invite.role).toBe("member");
    expect(invite.inviterDID).toBe(admin.did);
    expect(invite.nonce).toMatch(/^[0-9a-f]{32}$/);
    expect(invite.expiresAt).toBe(now + 60_000);
    expect(invite.sig).toBeTruthy();
    expect(invite.relays).toEqual(PARAMS.relays);

    // Unlike device pairing, an invite must never transfer key material.
    expect(invite).not.toHaveProperty("seed");
    expect(invite).not.toHaveProperty("seedB64");
    expect(invite).not.toHaveProperty("privateKey");
  });
});

describe("verifyInvite", () => {
  it("accepts a freshly minted, unexpired invite", async () => {
    const admin = await generateIdentity();
    const now = 1_000_000;
    const invite = await createInvite(admin, PARAMS, 60_000, () => now);
    await verifyInvite(invite, () => now); // no throw
  });

  it("rejects an expired invite", async () => {
    const admin = await generateIdentity();
    let now = 1_000_000;
    const invite = await createInvite(admin, PARAMS, 60_000, () => now);
    now += 60_001;
    await expect(verifyInvite(invite, () => now)).rejects.toMatchObject({
      code: "invite_expired",
    });
  });

  it("rejects an invite whose signed claims were tampered with", async () => {
    const admin = await generateIdentity();
    const now = 1_000_000;
    const invite = await createInvite(admin, PARAMS, 60_000, () => now);
    const tampered = { ...invite, role: "admin" }; // privilege escalation attempt
    await expect(verifyInvite(tampered, () => now)).rejects.toMatchObject({
      code: "invite_invalid",
    });
  });

  it("rejects an invite whose inviterDID does not match the signer", async () => {
    const admin = await generateIdentity();
    const impostor = await generateIdentity();
    const now = 1_000_000;
    const invite = await createInvite(admin, PARAMS, 60_000, () => now);
    const spoofed = { ...invite, inviterDID: impostor.did };
    await expect(verifyInvite(spoofed, () => now)).rejects.toBeInstanceOf(
      InviteError,
    );
  });
});

describe("encode/decode", () => {
  it("round-trips through base64url and still verifies", async () => {
    const admin = await generateIdentity();
    const now = 1_000_000;
    const invite = await createInvite(admin, PARAMS, 60_000, () => now);
    const decoded = decodeInvite(encodeInvite(invite));
    expect(decoded).toEqual(invite);
    await verifyInvite(decoded, () => now);
  });

  it("rejects malformed encoded text", () => {
    expect(() => decodeInvite("!!!not-base64!!!")).toThrow(InviteError);
  });
});

describe("invite URL", () => {
  it("builds a #/join?i=... URL and extracts the payload back", async () => {
    const admin = await generateIdentity();
    const now = 1_000_000;
    const invite = await createInvite(admin, PARAMS, 60_000, () => now);
    const url = buildInviteUrl("https://host.example/", invite);
    expect(url).toContain("#/join?i=");

    const decoded = decodeInvite(extractInviteParam(url));
    expect(decoded).toEqual(invite);
    await verifyInvite(decoded, () => now);
  });

  it("extractInviteParam returns raw payload when given just the payload", async () => {
    const admin = await generateIdentity();
    const invite = await createInvite(admin, PARAMS, 60_000);
    const encoded = encodeInvite(invite);
    expect(extractInviteParam(encoded)).toBe(encoded);
  });
});
