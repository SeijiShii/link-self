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
import { didToPeerId, parseToPublicKey } from "./did.js";
import { StreamReader } from "./framing.js";
import type { DuplexStream } from "./framing.js";
import type { PeerId, PrivateKey } from "@libp2p/interface";

/** Protocol ID for LinkSelf DID authentication. */
export const AUTH_PROTOCOL_ID = "/linkself/auth/1.0.0";

/** Byte length of the random challenge. */
export const CHALLENGE_SIZE = 32;

const MAX_SIGNATURE_SIZE = 1 << 20;

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
export async function respondToChallenge(stream: DuplexStream, privateKey: PrivateKey): Promise<void> {
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
export async function verifyChallenge(stream: DuplexStream, expectedDID: string, remotePeer?: PeerId): Promise<void> {
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
