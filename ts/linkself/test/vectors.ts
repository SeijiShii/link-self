/**
 * Golden interop vectors generated from the Go implementation
 * (core/internal/did, core/internal/auth, core/internal/envelope) with a
 * deterministic Ed25519 seed. If the Go wire format changes, regenerate.
 */
export const GOLDEN = {
  seedHex: "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
  pubKeyHex: "79b5562e8fe654f94078b112e8a98ba7901f853ae695bed7e0e3910bad049664",
  did: "did:key:z2DYcAYcYraFMjjqMjjR4rdxDRknbRpHoYRCWpKVhg9heA7",
  peerID: "12D3KooWJ1TsijH7H5F74hfAD5XishQz3sxrmAtVY37GtNd9CqYf",
  didKeyHex: "3c63295dd3d1efeead68378b1381ba655f957af0cd24cb5c86e7a3d4c354fd8e",
  challengeHex: "a0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebf",
  signatureHex:
    "c8d40ca74c08190fc337f66e9cd0ed9edc773dec61a6dfc857b6efbcb1178d6344f79a13ae525207dce0ff23472497d7c9c7e5ad8b47d1b2afb772b724add20c",
  envelopeJSON: '{"type":"groupshare","payload":"aGVsbG8gZW52ZWxvcGU="}',
} as const;

export function hexToBytes(hex: string): Uint8Array {
  const out = new Uint8Array(hex.length / 2);
  for (let i = 0; i < out.length; i++) {
    out[i] = parseInt(hex.slice(i * 2, i * 2 + 2), 16);
  }
  return out;
}

export function bytesToHex(bytes: Uint8Array): string {
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}
