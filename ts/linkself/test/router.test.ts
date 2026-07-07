import { describe, expect, it } from "vitest";
import { wrap } from "../src/envelope.js";
import { MessageRouter } from "../src/router.js";

const enc = new TextEncoder();
const dec = new TextDecoder();

describe("MessageRouter (mirroring router.go)", () => {
  function collect() {
    const calls: Array<{ handler: string; peerDID: string; payload: string }> = [];
    const router = new MessageRouter();
    for (const handler of ["onDeviceSync", "onGroupShare", "onSubAnnounce", "onMessage"] as const) {
      router[handler] = (peerDID, payload) => {
        calls.push({ handler, peerDID, payload: dec.decode(payload) });
      };
    }
    return { router, calls };
  }

  it("routes each envelope type to its handler", () => {
    const { router, calls } = collect();
    router.dispatch("did:key:zA", wrap("devicesync", enc.encode("d")));
    router.dispatch("did:key:zA", wrap("groupshare", enc.encode("g")));
    router.dispatch("did:key:zA", wrap("sub_announce", enc.encode("s")));
    expect(calls).toEqual([
      { handler: "onDeviceSync", peerDID: "did:key:zA", payload: "d" },
      { handler: "onGroupShare", peerDID: "did:key:zA", payload: "g" },
      { handler: "onSubAnnounce", peerDID: "did:key:zA", payload: "s" },
    ]);
  });

  it("routes plain (non-envelope) data to onMessage with the original bytes", () => {
    const { router, calls } = collect();
    router.dispatch("did:key:zB", enc.encode("raw bytes"));
    expect(calls).toEqual([{ handler: "onMessage", peerDID: "did:key:zB", payload: "raw bytes" }]);
  });

  it("routes unknown envelope types to onMessage with the unwrapped payload", () => {
    const { router, calls } = collect();
    const unknown = enc.encode('{"type":"future-thing","payload":"aGk="}');
    router.dispatch("did:key:zC", unknown);
    expect(calls).toEqual([{ handler: "onMessage", peerDID: "did:key:zC", payload: "hi" }]);
  });

  it("does nothing when no handler is registered", () => {
    const router = new MessageRouter();
    router.dispatch("did:key:zD", wrap("message", enc.encode("x"))); // no throw
  });
});
