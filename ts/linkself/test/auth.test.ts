import { generateKeyPairFromSeed } from "@libp2p/crypto/keys";
import { peerIdFromPublicKey } from "@libp2p/peer-id";
import { describe, expect, it } from "vitest";
import {
  AuthFailedError,
  mutualAuth,
  respondToChallenge,
  verifyChallenge,
  WrongPeerError,
} from "../src/auth.js";
import {
  generateIdentity,
  didToPeerId,
  identityFromPrivateKey,
} from "../src/did.js";
import { StreamReader } from "../src/framing.js";
import type { DuplexStream } from "../src/framing.js";
import { bytesToHex, GOLDEN, hexToBytes } from "./vectors.js";

/**
 * In-memory half-closable stream pair: what A sends, B reads (and vice
 * versa); closing A's writable end terminates B's read iterator.
 */
function streamPair(): [DuplexStream, DuplexStream] {
  type Waiter = (r: IteratorResult<Uint8Array>) => void;
  class Side implements DuplexStream {
    peer!: Side;
    private queue: Uint8Array[] = [];
    private waiters: Waiter[] = [];
    private ended = false;

    send(data: Uint8Array): boolean {
      this.peer.push(data);
      return true;
    }

    async close(): Promise<void> {
      this.peer.end();
    }

    private push(data: Uint8Array): void {
      const w = this.waiters.shift();
      if (w != null) w({ done: false, value: data });
      else this.queue.push(data);
    }

    private end(): void {
      this.ended = true;
      for (const w of this.waiters.splice(0))
        w({ done: true, value: undefined });
    }

    [Symbol.asyncIterator](): AsyncIterator<Uint8Array> {
      return {
        next: async (): Promise<IteratorResult<Uint8Array>> => {
          const queued = this.queue.shift();
          if (queued != null) return { done: false, value: queued };
          if (this.ended) return { done: true, value: undefined };
          return await new Promise<IteratorResult<Uint8Array>>((resolve) => {
            this.waiters.push(resolve);
          });
        },
      };
    }
  }
  const a = new Side();
  const b = new Side();
  a.peer = b;
  b.peer = a;
  return [a, b];
}

describe("auth challenge-response", () => {
  it("initiator and responder complete a round-trip", async () => {
    const responder = await generateIdentity();
    const [initiatorStream, responderStream] = streamPair();
    await Promise.all([
      verifyChallenge(
        initiatorStream,
        responder.did,
        didToPeerId(responder.did),
      ),
      respondToChallenge(responderStream, responder.privateKey),
    ]);
  });

  it("responder produces the same signature as Go for the golden challenge", async () => {
    const priv = await generateKeyPairFromSeed(
      "Ed25519",
      hexToBytes(GOLDEN.seedHex),
    );
    const [initiatorStream, responderStream] = streamPair();

    const respond = respondToChallenge(responderStream, priv);
    initiatorStream.send(hexToBytes(GOLDEN.challengeHex));
    await initiatorStream.close();
    await respond;

    const reader = new StreamReader(initiatorStream);
    const sigLen = await reader.readUint32();
    const sig = await reader.read(sigLen);
    expect(bytesToHex(sig)).toBe(GOLDEN.signatureHex);
  });

  it("fails when the responder signs with a different key", async () => {
    const expected = await generateIdentity();
    const actual = await generateIdentity();
    const [initiatorStream, responderStream] = streamPair();
    const respond = respondToChallenge(responderStream, actual.privateKey);
    await expect(
      verifyChallenge(initiatorStream, expected.did),
    ).rejects.toThrow(AuthFailedError);
    await respond.catch(() => {});
  });

  it("fails when the remote peer ID does not match the DID", async () => {
    const responder = await generateIdentity();
    const other = await generateIdentity();
    const otherPeer = peerIdFromPublicKey(other.publicKey);
    const [initiatorStream, responderStream] = streamPair();
    const respond = respondToChallenge(responderStream, responder.privateKey);
    await expect(
      verifyChallenge(initiatorStream, responder.did, otherPeer),
    ).rejects.toThrow(WrongPeerError);
    await respond.catch(() => {});
  });
});

describe("mutualAuth (multi-device, peerId decoupled from DID)", () => {
  it("both sides learn and verify the peer DID", async () => {
    const a = await generateIdentity();
    const b = await generateIdentity();
    const [sa, sb] = streamPair();
    const [ra, rb] = await Promise.all([
      mutualAuth(sa, a, true, b.did),
      mutualAuth(sb, b, false, a.did),
    ]);
    expect(ra).toBe(b.did);
    expect(rb).toBe(a.did);
  });

  it("works without a pre-known expected DID (learns it from the peer)", async () => {
    const a = await generateIdentity();
    const b = await generateIdentity();
    const [sa, sb] = streamPair();
    const [ra, rb] = await Promise.all([
      mutualAuth(sa, a, true),
      mutualAuth(sb, b, false),
    ]);
    expect(ra).toBe(b.did);
    expect(rb).toBe(a.did);
  });

  it("authenticates two devices that share one DID (different key objects)", async () => {
    // Same seed on both "devices" ⇒ same DID; each runs its own auth. The
    // transport key (peerId) is irrelevant here — that decoupling is the point.
    const seed = new Uint8Array(32).fill(7);
    const key = await generateKeyPairFromSeed("Ed25519", seed);
    const dev1 = identityFromPrivateKey(key);
    const dev2 = identityFromPrivateKey(key);
    expect(dev1.did).toBe(dev2.did);
    const [s1, s2] = streamPair();
    const [r1, r2] = await Promise.all([
      mutualAuth(s1, dev1, true, dev2.did),
      mutualAuth(s2, dev2, false, dev1.did),
    ]);
    expect(r1).toBe(dev1.did);
    expect(r2).toBe(dev1.did);
  });

  it("rejects a peer whose announced DID is not the expected one", async () => {
    const a = await generateIdentity();
    const b = await generateIdentity();
    const c = await generateIdentity();
    const [sa, sb] = streamPair();
    const init = mutualAuth(sa, a, true, c.did); // expect c, but b answers
    const resp = mutualAuth(sb, b, false);
    void resp.catch(() => {});
    await expect(init).rejects.toThrow(WrongPeerError);
    // The initiator aborted mid-handshake; closing its stream unblocks the
    // responder's pending read (in production node.ts closes on auth error).
    await sa.close();
  });

  it("rejects a forged signature (peer cannot prove its announced DID)", async () => {
    // Responder announces b.did but signs with a different key ⇒ verify fails.
    const a = await generateIdentity();
    const b = await generateIdentity();
    const imposter = await generateIdentity();
    const forged = { ...b, privateKey: imposter.privateKey };
    const [sa, sb] = streamPair();
    const init = mutualAuth(sa, a, true, b.did);
    const resp = mutualAuth(sb, forged, false);
    await expect(init).rejects.toThrow(AuthFailedError);
    await resp.catch(() => {});
  });
});
