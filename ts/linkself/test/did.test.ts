import { generateKeyPairFromSeed } from "@libp2p/crypto/keys";
import { describe, expect, it } from "vitest";
import {
  didToDHTKey,
  didToPeerId,
  generateIdentity,
  identityFromPrivateKey,
  InvalidDIDError,
  parseDID,
  parseToPublicKey,
  publicKeyToDID,
  rawPublicKeyToDID,
  UnsupportedKeyTypeError,
} from "../src/did.js";
import { bytesToHex, GOLDEN, hexToBytes } from "./vectors.js";

describe("did (golden vectors from Go implementation)", () => {
  it("derives the same DID as Go from the same key", async () => {
    const priv = await generateKeyPairFromSeed("Ed25519", hexToBytes(GOLDEN.seedHex));
    const id = identityFromPrivateKey(priv);
    expect(id.did).toBe(GOLDEN.did);
    expect(bytesToHex(id.publicKey.raw)).toBe(GOLDEN.pubKeyHex);
  });

  it("derives the same PeerId as Go", () => {
    expect(didToPeerId(GOLDEN.did).toString()).toBe(GOLDEN.peerID);
  });

  it("derives the same DHT key (sha256 of DID string) as Go", () => {
    expect(bytesToHex(didToDHTKey(GOLDEN.did))).toBe(GOLDEN.didKeyHex);
  });

  it("parses the DID back to the raw public key", () => {
    expect(bytesToHex(parseDID(GOLDEN.did))).toBe(GOLDEN.pubKeyHex);
    expect(bytesToHex(parseToPublicKey(GOLDEN.did).raw)).toBe(GOLDEN.pubKeyHex);
  });
});

describe("did (behaviour, mirroring did_test.go)", () => {
  it("same key produces the same DID (consistency)", async () => {
    const id = await generateIdentity();
    const again = identityFromPrivateKey(id.privateKey);
    expect(again.did).toBe(id.did);
  });

  it("round-trips generate → parse", async () => {
    const id = await generateIdentity();
    expect(bytesToHex(parseDID(id.did))).toBe(bytesToHex(id.publicKey.raw));
  });

  it("rejects missing prefix", () => {
    expect(() => parseDID("did:web:example.com")).toThrow(InvalidDIDError);
    expect(() => parseDID("z6Mk")).toThrow(InvalidDIDError);
  });

  it("rejects empty multibase part", () => {
    expect(() => parseDID("did:key:")).toThrow(InvalidDIDError);
  });

  it("rejects invalid multibase", () => {
    expect(() => parseDID("did:key:0Oli")).toThrow(InvalidDIDError);
  });

  it("rejects wrong multicodec", () => {
    // 0xec (x25519) instead of 0xed
    const raw = new Uint8Array(33).fill(7);
    raw[0] = 0xec;
    const bad = "did:key:" + encodeBase58btc(raw);
    expect(() => parseDID(bad)).toThrow(UnsupportedKeyTypeError);
  });

  it("rejects wrong key length", () => {
    const raw = new Uint8Array(17).fill(7);
    raw[0] = 0xed;
    const bad = "did:key:" + encodeBase58btc(raw);
    expect(() => parseDID(bad)).toThrow(InvalidDIDError);
  });

  it("rejects non-32-byte raw keys on encode", () => {
    expect(() => rawPublicKeyToDID(new Uint8Array(31))).toThrow(UnsupportedKeyTypeError);
  });

  it("publicKeyToDID matches rawPublicKeyToDID", async () => {
    const id = await generateIdentity();
    expect(publicKeyToDID(id.publicKey)).toBe(rawPublicKeyToDID(id.publicKey.raw));
  });
});

// tiny local base58btc encoder wrapper to build malformed DIDs in tests
import { base58btc } from "multiformats/bases/base58";
function encodeBase58btc(bytes: Uint8Array): string {
  return base58btc.encode(bytes);
}
