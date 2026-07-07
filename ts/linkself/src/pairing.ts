/**
 * Device pairing: a time-limited token securely transfers the user key from
 * an existing device to a new device (e.g. via QR code).
 * Port of core/internal/pairing (Go).
 *
 * The transferred key uses the libp2p protobuf private-key encoding
 * (Go crypto.MarshalPrivateKey ⇔ TS privateKeyToProtobuf), so pairing works
 * across the Go and TS implementations.
 */
import { privateKeyFromProtobuf, privateKeyToProtobuf } from "@libp2p/crypto/keys";
import { identityFromPrivateKey, UnsupportedKeyTypeError, type Identity } from "./did.js";

export type PairingErrorCode = "token_expired" | "token_invalid";

const PAIRING_ERROR_MESSAGES: Record<PairingErrorCode, string> = {
  token_expired: "pairing token expired",
  token_invalid: "pairing token invalid",
};

export class PairingError extends Error {
  constructor(readonly code: PairingErrorCode) {
    super(PAIRING_ERROR_MESSAGES[code]);
    this.name = "PairingError";
  }
}

/** A time-limited pairing token. */
export interface PairingToken {
  /** Random secret the new device must present (32 hex chars). */
  secret: string;
  /** Epoch milliseconds when this token becomes invalid. */
  expiresAt: number;
}

/** Generate a new pairing token valid for the given duration. */
export function createToken(ttlMs: number, now: () => number = Date.now): PairingToken {
  const b = new Uint8Array(16);
  globalThis.crypto.getRandomValues(b);
  return {
    secret: Array.from(b, (x) => x.toString(16).padStart(2, "0")).join(""),
    expiresAt: now() + ttlMs,
  };
}

/** Check that the provided secret matches and the token has not expired. */
export function validateToken(secret: string, token: PairingToken, now: () => number = Date.now): void {
  if (now() > token.expiresAt) {
    throw new PairingError("token_expired");
  }
  if (secret !== token.secret) {
    throw new PairingError("token_invalid");
  }
}

/** Serialize the user private key for transfer (libp2p protobuf encoding). */
export function exportPrivateKey(identity: Identity): Uint8Array {
  return privateKeyToProtobuf(identity.privateKey);
}

/** Reconstruct an Identity from an exported private key. */
export function importIdentity(data: Uint8Array): Identity {
  const priv = privateKeyFromProtobuf(data);
  if (priv.type !== "Ed25519") {
    throw new UnsupportedKeyTypeError();
  }
  return identityFromPrivateKey(priv);
}

/**
 * A single pairing attempt on the existing-device side. The existing device
 * creates a session, shares the token (e.g. via QR code), and the new
 * device calls complete() with the secret to receive the user key.
 */
export class PairingSession {
  readonly token: PairingToken;
  private readonly userKey: Uint8Array;
  private readonly now: () => number;

  constructor(identity: Identity, ttlMs: number, now: () => number = Date.now) {
    this.now = now;
    this.token = createToken(ttlMs, now);
    this.userKey = exportPrivateKey(identity);
  }

  /**
   * Validate the secret and return the exported user private key. The new
   * device reconstructs the identity with importIdentity(key).
   */
  complete(secret: string): Uint8Array {
    validateToken(secret, this.token, this.now);
    return this.userKey;
  }
}
