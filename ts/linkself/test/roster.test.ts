import { describe, expect, it } from "vitest";
import { generateIdentity } from "../src/did.js";
import {
  buildRoster,
  marshalRoster,
  rosterDeviceDIDs,
  rosterHasDevice,
  unmarshalRoster,
  verifyRoster,
  withDevice,
  withoutDevice,
  type DeviceEntry,
} from "../src/roster.js";

// Helper: a device DID is just another Ed25519 did:key (the device's transport key).
async function deviceEntry(label: string): Promise<DeviceEntry> {
  const id = await generateIdentity();
  return { deviceDID: id.did, label };
}

describe("device roster (signed by the user key)", () => {
  it("builds a roster the user key can verify", async () => {
    const user = await generateIdentity();
    const d1 = await deviceEntry("PC");
    const d2 = await deviceEntry("Phone");
    const roster = await buildRoster(user, [d1, d2]);

    expect(roster.userDID).toBe(user.did);
    expect(rosterDeviceDIDs(roster).sort()).toEqual(
      [d1.deviceDID, d2.deviceDID].sort(),
    );
    expect(await verifyRoster(roster)).toBe(true);
  });

  it("is order-independent (canonical encoding sorts devices)", async () => {
    const user = await generateIdentity();
    const d1 = await deviceEntry("A");
    const d2 = await deviceEntry("B");
    const r1 = await buildRoster(user, [d1, d2]);
    const r2 = await buildRoster(user, [d2, d1]);
    expect(r1.sig).toEqual(r2.sig);
  });

  it("de-duplicates devices by deviceDID (last wins)", async () => {
    const user = await generateIdentity();
    const d = await deviceEntry("old");
    const roster = await buildRoster(user, [d, { ...d, label: "new" }]);
    expect(roster.devices).toHaveLength(1);
    expect(roster.devices[0]!.label).toBe("new");
    expect(await verifyRoster(roster)).toBe(true);
  });

  it("rejects a roster signed by a different (impostor) key", async () => {
    const user = await generateIdentity();
    const impostor = await generateIdentity();
    const d1 = await deviceEntry("PC");
    const good = await buildRoster(user, [d1]);
    // Keep the signature but claim a different userDID ⇒ verify fails.
    const forged = { ...good, userDID: impostor.did };
    expect(await verifyRoster(forged)).toBe(false);
  });

  it("rejects a tampered device list", async () => {
    const user = await generateIdentity();
    const d1 = await deviceEntry("PC");
    const roster = await buildRoster(user, [d1]);
    const injected = await deviceEntry("rogue");
    const tampered = { ...roster, devices: [...roster.devices, injected] };
    expect(await verifyRoster(tampered)).toBe(false);
  });

  it("adds and removes devices, re-signing each time", async () => {
    const user = await generateIdentity();
    const d1 = await deviceEntry("PC");
    const d2 = await deviceEntry("Phone");

    let roster = await buildRoster(user, [d1]);
    roster = await withDevice(user, roster.devices, d2);
    expect(rosterHasDevice(roster, d2.deviceDID)).toBe(true);
    expect(await verifyRoster(roster)).toBe(true);

    roster = await withoutDevice(user, roster.devices, d1.deviceDID);
    expect(rosterHasDevice(roster, d1.deviceDID)).toBe(false);
    expect(rosterHasDevice(roster, d2.deviceDID)).toBe(true);
    expect(await verifyRoster(roster)).toBe(true);
  });

  it("round-trips through marshal/unmarshal", async () => {
    const user = await generateIdentity();
    const d1 = await deviceEntry("PC");
    const d2 = await deviceEntry("Phone");
    const roster = await buildRoster(user, [d1, d2]);

    const restored = unmarshalRoster(marshalRoster(roster));
    expect(restored.userDID).toBe(roster.userDID);
    expect(restored.devices).toEqual(roster.devices);
    // Byte equality (sign() may return a Buffer; unmarshal a plain Uint8Array).
    expect(Array.from(restored.sig)).toEqual(Array.from(roster.sig));
    expect(await verifyRoster(restored)).toBe(true);
  });
});
