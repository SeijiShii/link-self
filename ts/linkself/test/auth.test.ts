import { generateKeyPairFromSeed } from "@libp2p/crypto/keys";
import { peerIdFromPublicKey } from "@libp2p/peer-id";
import { describe, expect, it } from "vitest";
import {
  AuthFailedError,
  respondToChallenge,
  verifyChallenge,
  WrongPeerError,
} from "../src/auth.js";
import { generateIdentity, didToPeerId } from "../src/did.js";
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
      for (const w of this.waiters.splice(0)) w({ done: true, value: undefined });
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
      verifyChallenge(initiatorStream, responder.did, didToPeerId(responder.did)),
      respondToChallenge(responderStream, responder.privateKey),
    ]);
  });

  it("responder produces the same signature as Go for the golden challenge", async () => {
    const priv = await generateKeyPairFromSeed("Ed25519", hexToBytes(GOLDEN.seedHex));
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
    await expect(verifyChallenge(initiatorStream, expected.did)).rejects.toThrow(AuthFailedError);
    await respond.catch(() => {});
  });

  it("fails when the remote peer ID does not match the DID", async () => {
    const responder = await generateIdentity();
    const other = await generateIdentity();
    const otherPeer = peerIdFromPublicKey(other.publicKey);
    const [initiatorStream, responderStream] = streamPair();
    const respond = respondToChallenge(responderStream, responder.privateKey);
    await expect(verifyChallenge(initiatorStream, responder.did, otherPeer)).rejects.toThrow(
      WrongPeerError,
    );
    await respond.catch(() => {});
  });
});
