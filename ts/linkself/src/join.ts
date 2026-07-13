/**
 * Join handshake: the acceptance-side domain logic and wire codec for a new
 * DID joining a network via an invitation (invitation.ts).
 *
 * Network membership is held locally per node (network-concept.md §1-2), so
 * acceptance happens on an admin's node. The invitee dials an admin (via relay),
 * authenticates (transport layer), and presents a {@link JoinRequest} on the
 * `/linkself/join/1.0.0` stream. The admin runs {@link JoinService.accept} and
 * returns a {@link JoinResponse} carrying the updated network snapshot.
 *
 * This module is transport-free (pure logic + JSON codec), mirroring how the
 * domain layers (network.ts / groupshare.ts) are kept independent of node.ts /
 * client.ts. The live libp2p stream wiring is Slice 2b.
 *
 * Design: docs/spec/network-invitation.md §3
 */
import { type Invite, InviteError, verifyInvite } from "./invitation.js";
import {
  NetworkError,
  type NetworkService,
  type NetworkStore,
} from "./network.js";
import type { RoleDAG } from "./role.js";

/** libp2p protocol id for the join handshake. */
export const JOIN_PROTOCOL_ID = "/linkself/join/1.0.0";

/** What the invitee presents to an admin. */
export interface JoinRequest {
  v: 1;
  invite: Invite;
  /** The invitee's own DID (authenticated by the transport layer in Slice 2b). */
  inviteeDID: string;
  /** Human-facing name the invitee wants recorded. */
  displayName: string;
}

/** The network state handed back so the invitee can start syncing. */
export interface NetworkSnapshot {
  networkId: string;
  suiteId: string;
  members: string[];
  memberRoles: Record<string, string>;
}

export type JoinRejectCode =
  | "invite_expired"
  | "invite_invalid"
  | "network_not_found"
  | "inviter_not_admin"
  | "not_admin"
  | "invite_consumed"
  | "already_member";

export type JoinResponse =
  { ok: true; network: NetworkSnapshot } | { ok: false; code: JoinRejectCode };

/** Tracks consumed invite nonces so an invite is single-use per accepting node. */
export interface ConsumedNonceStore {
  has(nonce: string): Promise<boolean>;
  add(nonce: string): Promise<void>;
}

/** In-memory {@link ConsumedNonceStore} for tests / validation. */
export class MemConsumedNonceStore implements ConsumedNonceStore {
  private readonly seen = new Set<string>();
  async has(nonce: string): Promise<boolean> {
    return this.seen.has(nonce);
  }
  async add(nonce: string): Promise<void> {
    this.seen.add(nonce);
  }
}

/**
 * Acceptance-side logic run on an admin's node. Verifies the invite, checks the
 * issuer and the accepting node are both admins of the target network, enforces
 * single-use via the nonce store, then adds the invitee and returns a snapshot.
 */
export class JoinService {
  constructor(
    private readonly networks: NetworkService,
    private readonly store: NetworkStore,
    private readonly dag: RoleDAG,
    private readonly adminRole: string,
    private readonly nonces: ConsumedNonceStore,
    private readonly now: () => number = Date.now,
  ) {}

  /** `selfDID` is this (accepting) node's DID; it must be an admin member. */
  async accept(selfDID: string, req: JoinRequest): Promise<JoinResponse> {
    // 1. Signature + expiry.
    try {
      await verifyInvite(req.invite, this.now);
    } catch (err) {
      return { ok: false, code: (err as InviteError).code };
    }

    // 2. The target network must exist locally.
    const net = await this.store.getNetwork(req.invite.networkId);
    if (net == null) {
      return { ok: false, code: "network_not_found" };
    }

    // 3. The invite must be for this network's application (anti cross-app replay).
    if (req.invite.suiteId !== net.suiteId) {
      return { ok: false, code: "invite_invalid" };
    }

    // 4. Authority: the issuer must be (or have been) an admin member.
    const inviterRole = net.memberRoles[req.invite.inviterDID] ?? "";
    if (!this.dag.hasRole(inviterRole, this.adminRole)) {
      return { ok: false, code: "inviter_not_admin" };
    }

    // 5. The accepting node must itself be an admin (it performs the mutation).
    const selfRole = net.memberRoles[selfDID] ?? "";
    if (!this.dag.hasRole(selfRole, this.adminRole)) {
      return { ok: false, code: "not_admin" };
    }

    // 6. Single-use.
    if (await this.nonces.has(req.invite.nonce)) {
      return { ok: false, code: "invite_consumed" };
    }

    // 7. Add the invitee with the invited role.
    try {
      await this.networks.addMember(
        req.invite.networkId,
        selfDID,
        req.inviteeDID,
        req.invite.role,
      );
    } catch (err) {
      if (err instanceof NetworkError && err.code === "already_member") {
        return { ok: false, code: "already_member" };
      }
      throw err;
    }
    await this.nonces.add(req.invite.nonce);

    // 8. Snapshot the updated network for the invitee to bootstrap from.
    const updated = await this.store.getNetwork(req.invite.networkId);
    const snap = updated ?? net;
    return {
      ok: true,
      network: {
        networkId: snap.id,
        suiteId: snap.suiteId,
        members: snap.members,
        memberRoles: snap.memberRoles,
      },
    };
  }
}

// --- wire codec (JSON; transport adds length framing) ---

function encodeJson(value: unknown): Uint8Array {
  return new TextEncoder().encode(JSON.stringify(value));
}

export function encodeJoinRequest(req: JoinRequest): Uint8Array {
  return encodeJson(req);
}

export function decodeJoinRequest(bytes: Uint8Array): JoinRequest {
  const v = JSON.parse(new TextDecoder().decode(bytes)) as JoinRequest;
  if (
    v == null ||
    v.v !== 1 ||
    typeof v.inviteeDID !== "string" ||
    typeof v.displayName !== "string" ||
    typeof v.invite !== "object"
  ) {
    throw new Error("malformed join request");
  }
  return v;
}

export function encodeJoinResponse(res: JoinResponse): Uint8Array {
  return encodeJson(res);
}

export function decodeJoinResponse(bytes: Uint8Array): JoinResponse {
  const v = JSON.parse(new TextDecoder().decode(bytes)) as JoinResponse;
  if (v == null || typeof (v as { ok: unknown }).ok !== "boolean") {
    throw new Error("malformed join response");
  }
  return v;
}
