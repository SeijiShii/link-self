/**
 * did:key generation, parsing, and conversion for LinkSelf.
 * Wire-compatible port of core/internal/did (Go).
 *
 * NOTE: LinkSelf encodes the multicodec as a SINGLE byte 0xed, not the
 * standard did:key varint (0xed 0x01). This must match the Go implementation
 * exactly — do not "fix" it to the spec.
 */
import { generateKeyPair, publicKeyFromRaw } from "@libp2p/crypto/keys";
import { peerIdFromPublicKey } from "@libp2p/peer-id";
import { sha256 } from "@noble/hashes/sha2";
import { base58btc } from "multiformats/bases/base58";
import type { Ed25519PrivateKey, Ed25519PublicKey, PeerId, PublicKey } from "@libp2p/interface";

/** did:key method prefix. */
export const DID_PREFIX = "did:key:";

/** Multicodec identifier for Ed25519 public key (LinkSelf: single byte). */
export const MULTICODEC_ED25519_PUB = 0xed;

const ED25519_PUB_LENGTH = 32;

export class InvalidDIDError extends Error {
  constructor(message = "invalid did:key format") {
    super(message);
    this.name = "InvalidDIDError";
  }
}

export class UnsupportedKeyTypeError extends Error {
  constructor(message = "only Ed25519 did:key is supported") {
    super(message);
    this.name = "UnsupportedKeyTypeError";
  }
}

/**
 * An Ed25519 key pair and the derived DID string.
 * The private key never leaves the process; only the DID is shared.
 */
export interface Identity {
  privateKey: Ed25519PrivateKey;
  publicKey: Ed25519PublicKey;
  did: string;
}

/** Create a new Ed25519 identity and its did:key. */
export async function generateIdentity(): Promise<Identity> {
  const privateKey = await generateKeyPair("Ed25519");
  return identityFromPrivateKey(privateKey);
}

/** Build an Identity from an existing Ed25519 private key. */
export function identityFromPrivateKey(privateKey: Ed25519PrivateKey): Identity {
  return {
    privateKey,
    publicKey: privateKey.publicKey,
    did: publicKeyToDID(privateKey.publicKey),
  };
}

/**
 * Encode a public key as did:key
 * (multibase base58btc of: 0xed + raw 32 bytes).
 */
export function publicKeyToDID(pub: PublicKey): string {
  if (pub.type !== "Ed25519" || pub.raw.length !== ED25519_PUB_LENGTH) {
    throw new UnsupportedKeyTypeError();
  }
  return rawPublicKeyToDID(pub.raw);
}

/** Encode raw Ed25519 public key bytes as did:key. */
export function rawPublicKeyToDID(raw: Uint8Array): string {
  if (raw.length !== ED25519_PUB_LENGTH) {
    throw new UnsupportedKeyTypeError();
  }
  const payload = new Uint8Array(1 + raw.length);
  payload[0] = MULTICODEC_ED25519_PUB;
  payload.set(raw, 1);
  return DID_PREFIX + base58btc.encode(payload);
}

/**
 * Parse a did:key string and return the raw Ed25519 public key bytes
 * (32 bytes, multicodec stripped).
 */
export function parseDID(did: string): Uint8Array {
  if (!did.startsWith(DID_PREFIX)) {
    throw new InvalidDIDError();
  }
  const mb = did.slice(DID_PREFIX.length);
  if (mb === "") {
    throw new InvalidDIDError();
  }
  let data: Uint8Array;
  try {
    data = base58btc.decode(mb);
  } catch (err) {
    throw new InvalidDIDError(`multibase decode: ${String(err)}`);
  }
  if (data.length < 2) {
    throw new InvalidDIDError();
  }
  if (data[0] !== MULTICODEC_ED25519_PUB) {
    throw new UnsupportedKeyTypeError();
  }
  const raw = data.subarray(1);
  if (raw.length !== ED25519_PUB_LENGTH) {
    throw new InvalidDIDError();
  }
  return raw;
}

/** Parse a did:key string and return a libp2p PublicKey (Ed25519). */
export function parseToPublicKey(did: string): PublicKey {
  return publicKeyFromRaw(parseDID(did));
}

/** Convert a did:key string to a libp2p PeerId (for DHT FindPeer etc.). */
export function didToPeerId(did: string): PeerId {
  return peerIdFromPublicKey(parseToPublicKey(did));
}

/**
 * Stable byte key for DHT Provide/Find: SHA-256 of the DID string.
 * Port of did.DIDToPeerIDKey (Go).
 */
export function didToDHTKey(did: string): Uint8Array {
  return sha256(new TextEncoder().encode(did));
}
