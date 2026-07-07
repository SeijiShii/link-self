/**
 * Live interop test: authenticate against a real Go LinkSelf node
 * (core/cmd/poc-wsnode) in both directions:
 *
 *  1. js initiator → Go responder: verifyChallenge over /linkself/auth/1.0.0
 *  2. Go initiator → js responder: send "auth-me"; the Go node runs
 *     auth.RunInitiator toward us and reports "auth-ok" / "auth-fail:…"
 *
 * Requires `go` on PATH; skipped otherwise.
 */
import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import { fileURLToPath } from "node:url";
import { noise } from "@chainsafe/libp2p-noise";
import { yamux } from "@chainsafe/libp2p-yamux";
import { webSockets } from "@libp2p/websockets";
import { multiaddr } from "@multiformats/multiaddr";
import { createLibp2p, type Libp2p } from "libp2p";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { AUTH_PROTOCOL_ID, respondToChallenge, verifyChallenge } from "../src/auth.js";
import { generateIdentity, type Identity } from "../src/did.js";
import { encodeFrame, StreamReader } from "../src/framing.js";

const CORE_DIR = fileURLToPath(new URL("../../../core", import.meta.url));
const MSG_PROTOCOL_ID = "/linkself/msg/1.0.0";

const hasGo = spawnSync("go", ["version"], { stdio: "ignore" }).status === 0;

describe.skipIf(!hasGo)("auth interop with the Go node", () => {
  let goProc: ChildProcess;
  let goNode: { did: string; wsAddr: string };
  let node: Libp2p;
  let identity: Identity;
  const inbound: Array<(payload: Uint8Array) => void> = [];

  beforeAll(async () => {
    // Start the Go harness and read its node info line.
    goProc = spawn("go", ["run", "./cmd/poc-wsnode"], { cwd: CORE_DIR, stdio: ["ignore", "pipe", "pipe"] });
    goNode = await new Promise((resolve, reject) => {
      let buf = "";
      const t = setTimeout(() => reject(new Error("timed out waiting for Go node info")), 60_000);
      goProc.stdout!.on("data", (chunk: Buffer) => {
        buf += chunk.toString();
        const line = buf.split("\n")[0];
        if (line !== undefined && line.trim() !== "") {
          clearTimeout(t);
          resolve(JSON.parse(line));
        }
      });
      goProc.on("error", reject);
      goProc.on("exit", (code) => reject(new Error(`go harness exited early: ${code}`)));
    });

    identity = await generateIdentity();
    node = await createLibp2p({
      privateKey: identity.privateKey,
      transports: [webSockets()],
      connectionEncrypters: [noise()],
      streamMuxers: [yamux()],
    });
    // Serve the LinkSelf auth responder (what a browser leaf must do).
    await node.handle(AUTH_PROTOCOL_ID, (stream) => {
      void respondToChallenge(stream, identity.privateKey).catch((err) => {
        console.error("[js] respondToChallenge:", err);
      });
    });
    // Collect echo/auth-result replies from the Go node.
    await node.handle(MSG_PROTOCOL_ID, (stream) => {
      void new StreamReader(stream)
        .readFrame()
        .then(async (payload) => {
          await stream.close();
          inbound.shift()?.(payload);
        })
        .catch((err) => {
          console.error("[js] inbound frame:", err);
        });
    });
  }, 90_000);

  afterAll(async () => {
    await node?.stop();
    goProc?.kill("SIGTERM");
  });

  it("js initiator authenticates the Go responder (verifyChallenge)", async () => {
    const conn = await node.dial(multiaddr(goNode.wsAddr));
    const stream = await conn.newStream(AUTH_PROTOCOL_ID);
    await verifyChallenge(stream, goNode.did, conn.remotePeer);
  });

  it("Go initiator authenticates the js responder (auth-me round-trip)", async () => {
    const reply = new Promise<string>((resolve) => {
      inbound.push((payload) => resolve(new TextDecoder().decode(payload)));
    });
    const stream = await node.dialProtocol(multiaddr(goNode.wsAddr), MSG_PROTOCOL_ID);
    stream.send(encodeFrame(new TextEncoder().encode("auth-me")));
    await stream.close();
    expect(await reply).toBe("auth-ok");
  });
});
