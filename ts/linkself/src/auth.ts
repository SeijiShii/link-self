/**
 * Challenge-response authentication over a libp2p stream.
 * Wire-compatible port of core/internal/auth (Go):
 *
 *   initiator                         responder
 *     ── 32-byte random challenge ──▶
 *     ◀─ uint32 BE sig length ───────
 *     ◀─ Ed25519 signature ──────────
 *
 * The initiator verifies the signature against the public key encoded in the
 * expected DID, and that the remote peer ID matches the DID-derived peer ID.
 */
import { didToPeerId, parseToPublicKey, type Identity } from "./did.js";
import { StreamReader } from "./framing.js";
import type { DuplexStream } from "./framing.js";
import type { PeerId, PrivateKey } from "@libp2p/interface";

/** Protocol ID for LinkSelf DID authentication (one-way, peerId≡DID). */
export const AUTH_PROTOCOL_ID = "/linkself/auth/1.0.0";

/**
 * Protocol ID for mutual DID authentication (multi-device).
 * Unlike AUTH_PROTOCOL_ID this does NOT require the transport peer ID to equal
 * the DID-derived peer ID: the libp2p host key is a per-device transport key
 * and the shared DID key only signs the auth challenge. This decouples device
 * identity (peerId, authenticated by Noise) from user identity (DID,
 * authenticated by the challenge signature), so several devices sharing one
 * DID — each with its own transport key — can authenticate to each other.
 */
export const MUTUAL_AUTH_PROTOCOL_ID = "/linkself/mauth/1.0.0";

/** Byte length of the random challenge. */
export const CHALLENGE_SIZE = 32;

const MAX_SIGNATURE_SIZE = 1 << 20;
/** Upper bound on an announced DID string (bytes) — guards the length prefix. */
const MAX_DID_SIZE = 1 << 12;

export class AuthFailedError extends Error {
  constructor(message = "authentication failed") {
    super(message);
    this.name = "AuthFailedError";
  }
}

export class WrongPeerError extends Error {
  constructor(message = "peer ID does not match expected DID") {
    super(message);
    this.name = "WrongPeerError";
  }
}

/**
 * Responder side: read the challenge from the stream, sign it with our
 * private key, write the length-prefixed signature.
 * Port of auth.ChallengeResponse (Go).
 */
export async function respondToChallenge(
  stream: DuplexStream,
  privateKey: PrivateKey,
): Promise<void> {
  const reader = new StreamReader(stream);
  const challenge = await reader.read(CHALLENGE_SIZE);
  const sig = await privateKey.sign(challenge);
  const out = new Uint8Array(4 + sig.length);
  new DataView(out.buffer).setUint32(0, sig.length, false);
  out.set(sig, 4);
  stream.send(out);
  await stream.close();
}

/**
 * Initiator side: write a random challenge, read the signature, verify it
 * against the expected DID. If remotePeer is given, also verify it matches
 * the DID-derived peer ID. Port of auth.VerifyChallenge (Go).
 */
export async function verifyChallenge(
  stream: DuplexStream,
  expectedDID: string,
  remotePeer?: PeerId,
): Promise<void> {
  const publicKey = parseToPublicKey(expectedDID);
  const challenge = new Uint8Array(CHALLENGE_SIZE);
  globalThis.crypto.getRandomValues(challenge);

  stream.send(challenge);
  // Half-close our writable end (the Go responder reads exactly 32 bytes,
  // and the Go initiator does CloseWrite after the challenge).
  await stream.close();

  const reader = new StreamReader(stream);
  const sigLen = await reader.readUint32();
  if (sigLen > MAX_SIGNATURE_SIZE) {
    throw new AuthFailedError(`signature too large: ${sigLen}`);
  }
  const sig = await reader.read(sigLen);

  const ok = await publicKey.verify(challenge, sig);
  if (!ok) {
    throw new AuthFailedError();
  }
  if (remotePeer != null) {
    const expectedPeer = didToPeerId(expectedDID);
    if (!remotePeer.equals(expectedPeer)) {
      throw new WrongPeerError();
    }
  }
}

/** Encode `u32 BE didLen | did(utf8) | challenge(32)`. */
function encodeIntro(did: string, challenge: Uint8Array): Uint8Array {
  const didBytes = new TextEncoder().encode(did);
  const out = new Uint8Array(4 + didBytes.length + challenge.length);
  new DataView(out.buffer).setUint32(0, didBytes.length, false);
  out.set(didBytes, 4);
  out.set(challenge, 4 + didBytes.length);
  return out;
}

/** Read an intro frame (peer DID + its challenge). */
async function readIntro(
  reader: StreamReader,
): Promise<{ did: string; challenge: Uint8Array }> {
  const didLen = await reader.readUint32();
  if (didLen > MAX_DID_SIZE) {
    throw new AuthFailedError(`DID too large: ${didLen}`);
  }
  const did = new TextDecoder().decode(await reader.read(didLen));
  const challenge = await reader.read(CHALLENGE_SIZE);
  return { did, challenge };
}

/** Send `u32 BE sigLen | sig`. */
function sendSig(stream: DuplexStream, sig: Uint8Array): void {
  const out = new Uint8Array(4 + sig.length);
  new DataView(out.buffer).setUint32(0, sig.length, false);
  out.set(sig, 4);
  stream.send(out);
}

/** Read a length-prefixed signature. */
async function readSig(reader: StreamReader): Promise<Uint8Array> {
  const sigLen = await reader.readUint32();
  if (sigLen > MAX_SIGNATURE_SIZE) {
    throw new AuthFailedError(`signature too large: ${sigLen}`);
  }
  return await reader.read(sigLen);
}

/**
 * Mutual DID authentication over a libp2p stream (MUTUAL_AUTH_PROTOCOL_ID).
 * Both peers announce their DID and a random challenge, then each signs the
 * peer's challenge with its DID key; each verifies the peer's signature
 * against the peer's announced DID. The transport peer ID is NOT checked
 * against the DID (that binding is dropped for multi-device) — Noise already
 * authenticated the transport key, and the signature authenticates the DID.
 *
 * Strict alternation (initiator talks first in each phase) keeps the exchange
 * deadlock-free on a full-duplex stream. Returns the verified peer DID.
 *
 * @param initiator true on the dialing side, false on the responder side.
 * @param expectedPeerDID if given, the peer's announced DID must equal it.
 */
export async function mutualAuth(
  stream: DuplexStream,
  identity: Identity,
  initiator: boolean,
  expectedPeerDID?: string,
): Promise<string> {
  const reader = new StreamReader(stream);
  const myChallenge = new Uint8Array(CHALLENGE_SIZE);
  globalThis.crypto.getRandomValues(myChallenge);
  const myIntro = encodeIntro(identity.did, myChallenge);

  // Phase 1: exchange DID + challenge.
  let peer: { did: string; challenge: Uint8Array };
  if (initiator) {
    stream.send(myIntro);
    peer = await readIntro(reader);
  } else {
    peer = await readIntro(reader);
    stream.send(myIntro);
  }
  if (expectedPeerDID != null && peer.did !== expectedPeerDID) {
    throw new WrongPeerError(
      `expected DID ${expectedPeerDID}, got ${peer.did}`,
    );
  }

  // Phase 2: exchange signatures over the peer's challenge.
  const mySig = await identity.privateKey.sign(peer.challenge);
  let peerSig: Uint8Array;
  if (initiator) {
    sendSig(stream, mySig);
    peerSig = await readSig(reader);
  } else {
    peerSig = await readSig(reader);
    sendSig(stream, mySig);
  }

  const peerPub = parseToPublicKey(peer.did);
  if (!(await peerPub.verify(myChallenge, peerSig))) {
    throw new AuthFailedError();
  }
  return peer.did;
}
