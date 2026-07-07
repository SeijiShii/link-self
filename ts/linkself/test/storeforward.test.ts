import { describe, expect, it } from "vitest";
import { StoreForward } from "../src/storeforward.js";

const enc = new TextEncoder();
const dec = new TextDecoder();

describe("StoreForward (mirroring storeforward_test.go)", () => {
  it("queues per DID and reports pending counts", () => {
    const s = new StoreForward();
    expect(s.pendingCount("did:key:zA")).toBe(0);
    s.queue("did:key:zA", enc.encode("m1"));
    s.queue("did:key:zA", enc.encode("m2"));
    s.queue("did:key:zB", enc.encode("other"));
    expect(s.pendingCount("did:key:zA")).toBe(2);
    expect(s.pendingCount("did:key:zB")).toBe(1);
  });

  it("flushes all messages in order and clears the queue", async () => {
    const s = new StoreForward();
    s.queue("did:key:zA", enc.encode("m1"));
    s.queue("did:key:zA", enc.encode("m2"));
    const sent: string[] = [];
    const n = await s.flushForDID("did:key:zA", async (p) => {
      sent.push(dec.decode(p));
    });
    expect(n).toBe(2);
    expect(sent).toEqual(["m1", "m2"]);
    expect(s.pendingCount("did:key:zA")).toBe(0);
  });

  it("re-queues remaining messages on send failure", async () => {
    const s = new StoreForward();
    s.queue("did:key:zA", enc.encode("m1"));
    s.queue("did:key:zA", enc.encode("m2"));
    s.queue("did:key:zA", enc.encode("m3"));
    let calls = 0;
    await expect(
      s.flushForDID("did:key:zA", async () => {
        calls++;
        if (calls === 2) throw new Error("boom");
      }),
    ).rejects.toThrow("boom");
    // m1 sent, m2/m3 re-queued
    expect(s.pendingCount("did:key:zA")).toBe(2);
    const sent: string[] = [];
    await s.flushForDID("did:key:zA", async (p) => {
      sent.push(dec.decode(p));
    });
    expect(sent).toEqual(["m2", "m3"]);
  });

  it("flushing an unknown DID sends nothing", async () => {
    const s = new StoreForward();
    const n = await s.flushForDID("did:key:zNothing", async () => {
      throw new Error("should not be called");
    });
    expect(n).toBe(0);
  });
});
