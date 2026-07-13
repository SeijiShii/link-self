// Golden cross-implementation vector: the same fixed seeds must yield the same
// DIDs and (deterministic Ed25519) signature as the Go port
// (core/internal/roster/golden_test.go). This locks the roster wire format so a
// roster signed by one implementation verifies in the other.
import { generateKeyPairFromSeed } from "@libp2p/crypto/keys";
import { describe, expect, it } from "vitest";
import { identityFromPrivateKey, type Identity } from "../src/did.js";
import { buildRoster, verifyRoster } from "../src/roster.js";
import { bytesToHex } from "./vectors.js";

const GOLDEN_USER_DID =
  "did:key:z2DZjrAhKXqCQ2djr26Syq33DKt1YP5cUoAhxkiaeVUpBtX";
const GOLDEN_D1_DID = "did:key:z2DZ7WFjPGgZ5iteMKgdgLWu4AaSSADEdGRm6oBxB5NfFXH";
const GOLDEN_D2_DID = "did:key:z2DgPLAbK8rAzua3qFkqfS1Dnzd8BxxadcsVtghmB6AsPD2";
const GOLDEN_SIG_HEX =
  "3b69a9020a9396d37e3b86ba5784b3c12a8dbd83a6445d3a073a388dd1792facf5ec15e339cb637847b2c511294c3f0c63c1dc38687601fe1e8cac357c24ee04";

async function idFromSeed(seedByte: number): Promise<Identity> {
  const seed = new Uint8Array(32).fill(seedByte);
  return identityFromPrivateKey(await generateKeyPairFromSeed("Ed25519", seed));
}

describe("roster golden vector (Go/TS wire parity)", () => {
  it("derives the same DIDs and signature as the Go port", async () => {
    const user = await idFromSeed(0x01);
    const d1 = await idFromSeed(0x02);
    const d2 = await idFromSeed(0x03);

    expect(user.did).toBe(GOLDEN_USER_DID);
    expect(d1.did).toBe(GOLDEN_D1_DID);
    expect(d2.did).toBe(GOLDEN_D2_DID);

    const roster = await buildRoster(user, [
      { deviceDID: d1.did, label: "PC" },
      { deviceDID: d2.did, label: "Phone" },
    ]);
    expect(bytesToHex(roster.sig)).toBe(GOLDEN_SIG_HEX);
    expect(await verifyRoster(roster)).toBe(true);
  });
});
