import { describe, expect, it } from "vitest";
import { TYPE_GROUP_SHARE, TYPE_MESSAGE, unwrap, wrap } from "../src/envelope.js";
import { GOLDEN } from "./vectors.js";

const enc = new TextEncoder();
const dec = new TextDecoder();

describe("envelope (golden vector from Go implementation)", () => {
  it("wrap produces byte-identical JSON to Go", () => {
    const out = wrap(TYPE_GROUP_SHARE, enc.encode("hello envelope"));
    expect(dec.decode(out)).toBe(GOLDEN.envelopeJSON);
  });

  it("unwrap parses Go-produced JSON", () => {
    const { type, payload } = unwrap(enc.encode(GOLDEN.envelopeJSON));
    expect(type).toBe(TYPE_GROUP_SHARE);
    expect(dec.decode(payload)).toBe("hello envelope");
  });
});

describe("envelope (behaviour, mirroring envelope_test.go)", () => {
  it("round-trips arbitrary binary payloads", () => {
    const payload = new Uint8Array(256).map((_, i) => i);
    const { type, payload: got } = unwrap(wrap("devicesync", payload));
    expect(type).toBe("devicesync");
    expect(got).toEqual(payload);
  });

  it("falls back to message type for non-JSON data", () => {
    const data = enc.encode("plain text, not json");
    const { type, payload } = unwrap(data);
    expect(type).toBe(TYPE_MESSAGE);
    expect(payload).toEqual(data);
  });

  it("falls back to message type when type field is missing or empty", () => {
    const noType = enc.encode('{"payload":"aGk="}');
    expect(unwrap(noType).type).toBe(TYPE_MESSAGE);
    expect(unwrap(noType).payload).toEqual(noType);

    const emptyType = enc.encode('{"type":"","payload":"aGk="}');
    expect(unwrap(emptyType).type).toBe(TYPE_MESSAGE);
    expect(unwrap(emptyType).payload).toEqual(emptyType);
  });

  it("falls back to message type for JSON that is not an object", () => {
    const data = enc.encode('"just a string"');
    expect(unwrap(data).type).toBe(TYPE_MESSAGE);
    expect(unwrap(data).payload).toEqual(data);
  });

  it("handles empty payloads", () => {
    const { type, payload } = unwrap(wrap("sub_announce", new Uint8Array(0)));
    expect(type).toBe("sub_announce");
    expect(payload.length).toBe(0);
  });
});
