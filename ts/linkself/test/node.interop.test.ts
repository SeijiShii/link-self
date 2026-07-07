/**
 * Live interop: LinkSelfNode (TS leaf) against the real Go node.
 * Covers connectToAddr (dial + auth + store-and-forward flush), sendMessage,
 * and inbound routing of the Go node's echo replies.
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
import { generateIdentity, type Identity } from "../src/did.js";
import { LinkSelfNode } from "../src/node.js";

const CORE_DIR = fileURLToPath(new URL("../../../core", import.meta.url));
const hasGo = spawnSync("go", ["version"], { stdio: "ignore" }).status === 0;
const dec = new TextDecoder();
const enc = new TextEncoder();

describe.skipIf(!hasGo)("LinkSelfNode interop with the Go node", () => {
  let goProc: ChildProcess;
  let goNode: { did: string; wsAddr: string };
  let libp2p: Libp2p;
  let identity: Identity;
  let leaf: LinkSelfNode;
  const received: string[] = [];
  const waiters: Array<(msg: string) => void> = [];
  const authed: string[] = [];

  const nextMessage = async (): Promise<string> =>
    received.shift() ??
    (await new Promise<string>((resolve, reject) => {
      const t = setTimeout(() => reject(new Error("timed out waiting for message")), 10_000);
      waiters.push((m) => {
        clearTimeout(t);
        resolve(m);
      });
    }));

  beforeAll(async () => {
    goProc = spawn("go", ["run", "./cmd/poc-wsnode"], {
      cwd: CORE_DIR,
      stdio: ["ignore", "pipe", "pipe"],
    });
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
    libp2p = await createLibp2p({
      privateKey: identity.privateKey,
      transports: [webSockets()],
      connectionEncrypters: [noise()],
      streamMuxers: [yamux()],
    });
    leaf = new LinkSelfNode(libp2p, identity);
    leaf.setOnMessage((_peerDID, payload) => {
      const msg = dec.decode(payload);
      const w = waiters.shift();
      if (w != null) w(msg);
      else received.push(msg);
    });
    leaf.setOnAuthSuccess((peerDID) => {
      authed.push(peerDID);
    });
    await leaf.start();
  }, 90_000);

  afterAll(async () => {
    await libp2p?.stop();
    goProc?.kill("SIGTERM");
  });

  it("queues messages for an unconnected peer, then flushes them on connect", async () => {
    await leaf.sendMessage(goNode.did, enc.encode("queued before connect"));
    expect(leaf.storeForward.pendingCount(goNode.did)).toBe(1);

    await leaf.connectToAddr(goNode.did, multiaddr(goNode.wsAddr));
    expect(leaf.storeForward.pendingCount(goNode.did)).toBe(0);
    expect(authed).toContain(goNode.did);

    // The Go echo node replies "echo:<payload>" to the flushed message.
    expect(await nextMessage()).toBe("echo:queued before connect");
  });

  it("sends directly over the established connection", async () => {
    await leaf.sendMessage(goNode.did, enc.encode("direct message"));
    expect(await nextMessage()).toBe("echo:direct message");
  });
});
