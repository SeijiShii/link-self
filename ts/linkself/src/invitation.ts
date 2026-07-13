/**
 * Network invitation: a time-limited, signed capability that lets a NEW DID
 * join an existing network with an assigned role.
 *
 * Unlike device pairing (pairing.ts), an invitation transfers **no key
 * material** — the invitee keeps (or creates) their own identity. An admin of
 * the network issues the invite, signing the claims with their user key. The
 * invitee presents it over the join handshake (`/linkself/join/1.0.0`), and any
 * admin member can verify the issuer's signature and add the invitee to the
 * network (network membership is held locally per node, so acceptance happens
 * on an admin's node — see network-concept.md §1-2).
 *
 * The invite is delivered out-of-band (URL / QR). It is a bearer capability
 * until it expires, so keep the TTL short-ish (home-visit-suite uses 3 days)
 * and treat the nonce as the handle for single-use tracking / revocation on the
 * accepting node.
 *
 * Wire format is JSON (canonical field order for the signed claims). This is a
 * TS-first protocol; a Go counterpart can mirror this canonicalization when the
 * daemon grows a join handshake (cf. groupshare interop note in
 * docs/spec/browser-pwa-support.md §7).
 *
 * Design: docs/spec/network-invitation.md
 */
import { parseToPublicKey, type Identity } from "./did.js";

export type InviteErrorCode = "invite_expired" | "invite_invalid";

const INVITE_ERROR_MESSAGES: Record<InviteErrorCode, string> = {
  invite_expired: "invitation expired",
  invite_invalid: "invitation invalid",
};

export class InviteError extends Error {
  constructor(readonly code: InviteErrorCode) {
    super(INVITE_ERROR_MESSAGES[code]);
    this.name = "InviteError";
  }
}

/** The signed part of an invitation — everything the signature covers. */
export interface InviteClaims {
  v: 1;
  /** Network the invitee is being added to. */
  networkId: string;
  /** Application / suite identifier (guards against cross-app replay). */
  suiteId: string;
  /** Role assigned to the invitee on acceptance (resolved via the role DAG). */
  role: string;
  /** DID of the issuing admin; the signer and the DID the invitee authenticates. */
  inviterDID: string;
  /** Unique invite id (32 hex). Handle for single-use tracking / revocation. */
  nonce: string;
  /** Epoch milliseconds when this invite becomes invalid. */
  expiresAt: number;
}

/**
 * Full invitation payload = signed claims + signature + transport hints.
 *
 * `relays` are outside the signature on purpose: they are reachability hints
 * (circuit-relay multiaddrs), not authorization. A swapped relay only affects
 * how the invitee reaches an admin — it cannot read E2E content and the invitee
 * still authenticates the admin by `inviterDID` during the handshake.
 */
export interface Invite extends InviteClaims {
  /** base64 Ed25519 signature over canonicalClaims(claims) by the inviter. */
  sig: string;
  /** Circuit-relay multiaddrs to reach an admin (bootstrap for the invitee). */
  relays: string[];
}

/** Parameters the issuer supplies; the rest (nonce/expiresAt/sig) is derived. */
export interface InviteParams {
  networkId: string;
  suiteId: string;
  role: string;
  relays: string[];
}

/** Canonical bytes of the signed claims (fixed field order). */
function canonicalClaims(c: InviteClaims): Uint8Array {
  return new TextEncoder().encode(
    JSON.stringify({
      v: c.v,
      networkId: c.networkId,
      suiteId: c.suiteId,
      role: c.role,
      inviterDID: c.inviterDID,
      nonce: c.nonce,
      expiresAt: c.expiresAt,
    }),
  );
}

function claimsOf(invite: Invite): InviteClaims {
  return {
    v: invite.v,
    networkId: invite.networkId,
    suiteId: invite.suiteId,
    role: invite.role,
    inviterDID: invite.inviterDID,
    nonce: invite.nonce,
    expiresAt: invite.expiresAt,
  };
}

/** 32-hex random nonce (16 bytes), mirroring the pairing token secret. */
function randomNonce(): string {
  const b = new Uint8Array(16);
  globalThis.crypto.getRandomValues(b);
  return Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
}

function bytesToB64(b: Uint8Array): string {
  let s = "";
  for (const x of b) s += String.fromCharCode(x);
  return btoa(s);
}

function b64ToBytes(s: string): Uint8Array {
  const bin = atob(s);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

function toBase64Url(b64: string): string {
  return b64.replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function fromBase64Url(s: string): string {
  const b64 = s.replace(/-/g, "+").replace(/_/g, "/");
  const pad = b64.length % 4 === 0 ? "" : "=".repeat(4 - (b64.length % 4));
  return b64 + pad;
}

/**
 * Issue a signed invitation valid for `ttlMs`. `inviter` must be an admin of
 * the network; acceptance re-checks that on the accepting node.
 */
export async function createInvite(
  inviter: Identity,
  params: InviteParams,
  ttlMs: number,
  now: () => number = Date.now,
): Promise<Invite> {
  const claims: InviteClaims = {
    v: 1,
    networkId: params.networkId,
    suiteId: params.suiteId,
    role: params.role,
    inviterDID: inviter.did,
    nonce: randomNonce(),
    expiresAt: now() + ttlMs,
  };
  const sig = await inviter.privateKey.sign(canonicalClaims(claims));
  return { ...claims, sig: bytesToB64(sig), relays: [...params.relays] };
}

/**
 * Verify signature and expiry. Throws {@link InviteError}. Does NOT check
 * whether `inviterDID` is actually an admin member — that is the accepting
 * node's responsibility against its local network state.
 */
export async function verifyInvite(
  invite: Invite,
  now: () => number = Date.now,
): Promise<void> {
  if (now() > invite.expiresAt) {
    throw new InviteError("invite_expired");
  }
  let ok = false;
  try {
    const pub = parseToPublicKey(invite.inviterDID);
    ok = await pub.verify(canonicalClaims(claimsOf(invite)), b64ToBytes(invite.sig));
  } catch {
    throw new InviteError("invite_invalid");
  }
  if (!ok) {
    throw new InviteError("invite_invalid");
  }
}

/** Encode an invite as URL/QR-safe text (base64url of the JSON). */
export function encodeInvite(invite: Invite): string {
  const bytes = new TextEncoder().encode(JSON.stringify(invite));
  let binary = "";
  for (const b of bytes) binary += String.fromCharCode(b);
  return toBase64Url(btoa(binary));
}

/** Decode invite text (base64url JSON) back into an {@link Invite}. */
export function decodeInvite(text: string): Invite {
  let value: unknown;
  try {
    const binary = atob(fromBase64Url(text.trim()));
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    value = JSON.parse(new TextDecoder().decode(bytes));
  } catch {
    throw new InviteError("invite_invalid");
  }
  if (!isInvite(value)) {
    throw new InviteError("invite_invalid");
  }
  return value;
}

/**
 * Build the join URL that carries the invite in the fragment (never sent to a
 * server): `${base}/#/join?i=...`.
 */
export function buildInviteUrl(baseUrl: string, invite: Invite): string {
  const base = baseUrl.replace(/#.*$/, "").replace(/\/+$/, "");
  return `${base}/#/join?i=${encodeInvite(invite)}`;
}

/**
 * Extract the invite payload from a full join URL, its fragment, or the raw
 * payload string. Returns the `i=` value if present, otherwise the input.
 */
export function extractInviteParam(input: string): string {
  const s = input.trim();
  const m = s.match(/[?&]i=([^&\s]+)/);
  return m ? m[1] : s;
}

function isInvite(v: unknown): v is Invite {
  if (typeof v !== "object" || v === null) return false;
  const p = v as Record<string, unknown>;
  return (
    p.v === 1 &&
    typeof p.networkId === "string" &&
    typeof p.suiteId === "string" &&
    typeof p.role === "string" &&
    typeof p.inviterDID === "string" &&
    typeof p.nonce === "string" &&
    typeof p.expiresAt === "number" &&
    typeof p.sig === "string" &&
    Array.isArray(p.relays) &&
    p.relays.every((r) => typeof r === "string")
  );
}
