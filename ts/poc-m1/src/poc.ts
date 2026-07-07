/**
 * M1 interop PoC: dial a Go LinkSelf node from js-libp2p over WebSocket
 * (and optionally via Circuit Relay v2), exchange /linkself/msg/1.0.0 frames.
 *
 * Usage: tsx src/poc.ts <echo-ws-multiaddr> [leaf-circuit-multiaddr]
 *
 * The Go side (core/cmd/poc-wsnode) echoes every message back to the sender
 * as "<role>:<payload>" on a fresh stream — so this script proves, in both
 * directions: multistream-select, Noise handshake, yamux muxing, and the
 * LinkSelf uint32-BE length-prefixed message framing.
 */
import { noise } from "@chainsafe/libp2p-noise";
import { yamux } from "@chainsafe/libp2p-yamux";
import { circuitRelayTransport } from "@libp2p/circuit-relay-v2";
import { identify } from "@libp2p/identify";
import { webSockets } from "@libp2p/websockets";
import { multiaddr } from "@multiformats/multiaddr";
import { createLibp2p } from "libp2p";
import type { Stream } from "@libp2p/interface";

const PROTOCOL = "/linkself/msg/1.0.0";
const REPLY_TIMEOUT_MS = 10_000;

/** LinkSelf message framing: uint32 big-endian length + payload. */
function encodeFrame(payload: Uint8Array): Uint8Array {
  const frame = new Uint8Array(4 + payload.length);
  new DataView(frame.buffer).setUint32(0, payload.length, false);
  frame.set(payload, 4);
  return frame;
}

/** Read one length-prefixed frame from a stream (collects chunks). */
async function readFrame(stream: Stream): Promise<Uint8Array> {
  const chunks: Uint8Array[] = [];
  let total = 0;
  for await (const chunk of stream) {
    const bytes = chunk instanceof Uint8Array ? chunk : chunk.subarray();
    chunks.push(bytes);
    total += bytes.length;
    if (total >= 4) {
      const head = concat(chunks, total);
      const len = new DataView(head.buffer, head.byteOffset).getUint32(
        0,
        false,
      );
      if (total >= 4 + len) {
        return head.subarray(4, 4 + len);
      }
    }
  }
  throw new Error(
    `stream ended before a full frame arrived (got ${total} bytes)`,
  );
}

function concat(chunks: Uint8Array[], total: number): Uint8Array {
  const out = new Uint8Array(total);
  let off = 0;
  for (const c of chunks) {
    out.set(c, off);
    off += c.length;
  }
  return out;
}

async function main(): Promise<void> {
  const [echoAddr, circuitAddr] = process.argv.slice(2);
  if (!echoAddr) {
    console.error(
      "usage: tsx src/poc.ts <echo-ws-multiaddr> [leaf-circuit-multiaddr]",
    );
    process.exit(2);
  }

  const node = await createLibp2p({
    transports: [webSockets(), circuitRelayTransport()],
    connectionEncrypters: [noise()],
    streamMuxers: [yamux()],
    services: { identify: identify() },
  });
  console.log(`[js] peer id: ${node.peerId.toString()}`);

  // Collect replies: the Go node opens a NEW stream back to us for the echo.
  const replies: Uint8Array[] = [];
  const waiters: Array<(v: Uint8Array) => void> = [];
  await node.handle(
    PROTOCOL,
    (stream) => {
      void readFrame(stream)
        .then(async (payload) => {
          await stream.close();
          const w = waiters.shift();
          if (w != null) {
            w(payload);
          } else {
            replies.push(payload);
          }
        })
        .catch((err) => {
          console.error("[js] inbound frame error:", err);
        });
    },
    // Echo replies from a relay-only peer arrive over a limited connection.
    { runOnLimitedConnection: true },
  );

  const nextReply = async (): Promise<Uint8Array> => {
    const queued = replies.shift();
    if (queued != null) return queued;
    return await new Promise<Uint8Array>((resolve, reject) => {
      const t = setTimeout(() => {
        reject(new Error("timed out waiting for echo reply"));
      }, REPLY_TIMEOUT_MS);
      waiters.push((v) => {
        clearTimeout(t);
        resolve(v);
      });
    });
  };

  const dec = new TextDecoder();
  const enc = new TextEncoder();

  const roundtrip = async (
    addr: string,
    message: string,
    expected: string,
    label: string,
  ): Promise<void> => {
    console.log(`[js] dialing ${label}: ${addr}`);
    const stream = await node.dialProtocol(multiaddr(addr), PROTOCOL, {
      // Relayed (Circuit Relay v2) connections are "limited"; allow our
      // protocol on them, mirroring WithAllowLimitedConn on the Go side.
      runOnLimitedConnection: true,
    });
    stream.send(encodeFrame(enc.encode(message)));
    await stream.close();
    const reply = dec.decode(await nextReply());
    if (reply !== expected) {
      throw new Error(
        `${label}: got ${JSON.stringify(reply)}, want ${JSON.stringify(expected)}`,
      );
    }
    console.log(`[js] ${label} OK: ${JSON.stringify(reply)}`);
  };

  try {
    await roundtrip(
      echoAddr,
      "hello from js-libp2p",
      "echo:hello from js-libp2p",
      "direct-ws",
    );
    if (circuitAddr != null) {
      await roundtrip(
        circuitAddr,
        "hello via relay",
        "leaf:hello via relay",
        "circuit-relay",
      );
    }
    console.log("[js] PoC PASSED");
    await node.stop();
    process.exit(0);
  } catch (err) {
    console.error("[js] PoC FAILED:", err);
    await node.stop();
    process.exit(1);
  }
}

void main();
