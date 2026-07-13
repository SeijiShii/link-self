/**
 * Message envelope: multiplexes DeviceSync / GroupShare / plain messages over
 * the same transport. Wire-compatible port of core/internal/envelope (Go):
 * JSON `{"type": "...", "payload": "<base64>"}` — Go's encoding/json encodes
 * []byte as standard base64 with padding.
 */

export type EnvelopeType =
  "devicesync" | "groupshare" | "sub_announce" | "network_meta" | "message";

export const TYPE_DEVICE_SYNC: EnvelopeType = "devicesync";
export const TYPE_GROUP_SHARE: EnvelopeType = "groupshare";
export const TYPE_SUB_ANNOUNCE: EnvelopeType = "sub_announce";
/** Network membership snapshot broadcast (TS-first; Go falls back to message). */
export const TYPE_NETWORK_META: EnvelopeType = "network_meta";
export const TYPE_MESSAGE: EnvelopeType = "message";

/** Create an envelope with the given type and payload, returning JSON bytes. */
export function wrap(type: EnvelopeType, payload: Uint8Array): Uint8Array {
  const json = JSON.stringify({ type, payload: bytesToBase64(payload) });
  return new TextEncoder().encode(json);
}

/**
 * Extract the type and payload from an envelope.
 * If the data is not a valid envelope, returns TYPE_MESSAGE and the original
 * data (matching the Go implementation's fallback behaviour).
 */
export function unwrap(data: Uint8Array): {
  type: EnvelopeType;
  payload: Uint8Array;
} {
  let parsed: unknown;
  try {
    parsed = JSON.parse(new TextDecoder().decode(data));
  } catch {
    return { type: TYPE_MESSAGE, payload: data };
  }
  if (parsed === null || typeof parsed !== "object") {
    return { type: TYPE_MESSAGE, payload: data };
  }
  const env = parsed as { type?: unknown; payload?: unknown };
  if (typeof env.type !== "string" || env.type === "") {
    return { type: TYPE_MESSAGE, payload: data };
  }
  const payload =
    typeof env.payload === "string"
      ? base64ToBytes(env.payload)
      : new Uint8Array(0);
  return { type: env.type as EnvelopeType, payload };
}

function bytesToBase64(bytes: Uint8Array): string {
  let bin = "";
  for (const b of bytes) {
    bin += String.fromCharCode(b);
  }
  return btoa(bin);
}

function base64ToBytes(b64: string): Uint8Array {
  const bin = atob(b64);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) {
    out[i] = bin.charCodeAt(i);
  }
  return out;
}
