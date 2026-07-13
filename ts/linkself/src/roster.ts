/**
 * Device roster: the abstraction that unifies a user's devices.
 *
 * Two-layer identity model (data-sync-decisions §4.2 P3): each device has its
 * own device DID (its libp2p transport key, so peerId ≡ deviceDID), while the
 * user DID is the account visible to the network. A roster binds them: it lists
 * the user's device DIDs and is signed by the USER key, so anyone holding the
 * user DID (its public key) can verify which devices legitimately belong to
 * that user.
 *
 * This lets device-to-device trust skip a challenge-response: Noise already
 * authenticates the transport key (= peer's device DID), and roster membership
 * (verified against the user key) proves that device DID is a sibling of the
 * same user. Routing/app layers operate on the user DID; the transport layer
 * on device DIDs; the roster bridges the two.
 */
import { parseToPublicKey, type Identity } from "./did.js";

/** One device in a user's roster. */
export interface DeviceEntry {
  /** The device's own did:key (its libp2p transport identity). */
  deviceDID: string;
  /** Human-readable label (e.g. "PC", "Phone"). Empty string if unset. */
  label: string;
}

/** A user's device list, signed by the user key. */
export interface SignedRoster {
  /** The user (account) DID — the signer, visible to the network. */
  userDID: string;
  /** All device DIDs belonging to this user. */
  devices: DeviceEntry[];
  /** Ed25519 signature by the user key over the canonical encoding. */
  sig: Uint8Array;
}

/**
 * Deterministic byte encoding signed by the user key. Devices are sorted by
 * deviceDID so the same roster always yields the same bytes regardless of
 * insertion order.
 */
function canonicalBytes(userDID: string, devices: DeviceEntry[]): Uint8Array {
  const sorted = [...devices].sort((a, b) =>
    a.deviceDID < b.deviceDID ? -1 : a.deviceDID > b.deviceDID ? 1 : 0,
  );
  const obj = {
    v: 1,
    userDID,
    devices: sorted.map((d) => ({ deviceDID: d.deviceDID, label: d.label })),
  };
  return new TextEncoder().encode(JSON.stringify(obj));
}

/** Build and sign a roster with the user identity. Devices are de-duplicated by deviceDID. */
export async function buildRoster(
  userIdentity: Identity,
  devices: DeviceEntry[],
): Promise<SignedRoster> {
  const deduped = dedupeByDeviceDID(devices);
  const sig = await userIdentity.privateKey.sign(
    canonicalBytes(userIdentity.did, deduped),
  );
  return { userDID: userIdentity.did, devices: deduped, sig };
}

/** Verify the roster signature against its own userDID's public key. */
export async function verifyRoster(roster: SignedRoster): Promise<boolean> {
  try {
    const pub = parseToPublicKey(roster.userDID);
    return await pub.verify(
      canonicalBytes(roster.userDID, roster.devices),
      roster.sig,
    );
  } catch {
    return false;
  }
}

/** The device DIDs in the roster. */
export function rosterDeviceDIDs(roster: SignedRoster): string[] {
  return roster.devices.map((d) => d.deviceDID);
}

/** Whether a device DID is a member of the roster. */
export function rosterHasDevice(
  roster: SignedRoster,
  deviceDID: string,
): boolean {
  return roster.devices.some((d) => d.deviceDID === deviceDID);
}

/**
 * Return a new signed roster with `entry` added or its label updated. Requires
 * the user identity (every device holds the user key after pairing, so any
 * device can amend and re-sign the roster).
 */
export async function withDevice(
  userIdentity: Identity,
  devices: DeviceEntry[],
  entry: DeviceEntry,
): Promise<SignedRoster> {
  const next = devices.filter((d) => d.deviceDID !== entry.deviceDID);
  next.push(entry);
  return buildRoster(userIdentity, next);
}

/** Return a new signed roster with the given device removed (roster revocation). */
export async function withoutDevice(
  userIdentity: Identity,
  devices: DeviceEntry[],
  deviceDID: string,
): Promise<SignedRoster> {
  return buildRoster(
    userIdentity,
    devices.filter((d) => d.deviceDID !== deviceDID),
  );
}

function dedupeByDeviceDID(devices: DeviceEntry[]): DeviceEntry[] {
  const byDID = new Map<string, DeviceEntry>();
  for (const d of devices) byDID.set(d.deviceDID, d); // last wins
  return [...byDID.values()];
}

/* ---- Wire encoding (for storage in MyDB / transfer) ---- */

/** Serialize a roster to JSON bytes (sig as base64). */
export function marshalRoster(roster: SignedRoster): Uint8Array {
  const obj = {
    userDID: roster.userDID,
    devices: roster.devices,
    sig: bytesToBase64(roster.sig),
  };
  return new TextEncoder().encode(JSON.stringify(obj));
}

/** Parse a roster from JSON bytes produced by marshalRoster. */
export function unmarshalRoster(data: Uint8Array): SignedRoster {
  const p = JSON.parse(new TextDecoder().decode(data)) as {
    userDID: string;
    devices: DeviceEntry[];
    sig: string;
  };
  return {
    userDID: p.userDID,
    devices: p.devices ?? [],
    sig: base64ToBytes(p.sig),
  };
}

function bytesToBase64(bytes: Uint8Array): string {
  let s = "";
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s);
}

function base64ToBytes(b64: string): Uint8Array {
  const s = atob(b64);
  const out = new Uint8Array(s.length);
  for (let i = 0; i < s.length; i++) out[i] = s.charCodeAt(i);
  return out;
}
