/**
 * Live interop: TS LinkSelfClient ↔ Go groupshare layer (core/cmd/poc-gsnode).
 * Verifies SharedRecord exchange in both directions over the real wire:
 *   Go put → broadcast → TS handleIncoming applies (LWW)
 *   TS put → broadcast → Go handleIncoming applies (checked via "check:<id>")
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
import { LinkSelfClient } from "../src/client.js";
import { generateIdentity, type Identity } from "../src/did.js";
import type { Network, NetworkStore } from "../src/network.js";

const CORE_DIR = fileURLToPath(new URL("../../../core", import.meta.url));
const hasGo = spawnSync("go", ["version"], { stdio: "ignore" }).status === 0;
const enc = new TextEncoder();
const dec = new TextDecoder();

class FixedNetworkStore implements NetworkStore {
  constructor(private readonly network: Network) {}
  async createNetwork(): Promise<string> {
    return this.network.id;
  }
  async getNetwork(id: string): Promise<Network | null> {
    return id === this.network.id ? this.network : null;
  }
  async updateNetwork(): Promise<void> {}
  async deleteNetwork(): Promise<void> {}
  async putNetwork(): Promise<void> {}
  async listForMember(): Promise<string[]> {
    return [this.network.id];
  }
}

async function eventually<T>(
  fn: () => Promise<T | null | false>,
  timeoutMs = 15_000,
): Promise<T> {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    const v = await fn();
    if (v != null && v !== false) return v;
    if (Date.now() > deadline) throw new Error("condition not met in time");
    await new Promise((r) => setTimeout(r, 150));
  }
}

describe.skipIf(!hasGo)("groupshare interop with the Go layer", () => {
  let goProc: ChildProcess;
  let goNode: { did: string; wsAddr: string };
  let libp2p: Libp2p;
  let identity: Identity;
  let client: LinkSelfClient;
  const plainMessages: string[] = [];

  beforeAll(async () => {
    identity = await generateIdentity();

    goProc = spawn("go", ["run", "./cmd/poc-gsnode", "-peer", identity.did], {
      cwd: CORE_DIR,
      stdio: ["ignore", "pipe", "pipe"],
    });
    goNode = await new Promise((resolve, reject) => {
      let buf = "";
      const t = setTimeout(
        () => reject(new Error("timed out waiting for Go node info")),
        60_000,
      );
      goProc.stdout!.on("data", (chunk: Buffer) => {
        buf += chunk.toString();
        const line = buf.split("\n")[0];
        if (line !== undefined && line.trim() !== "") {
          clearTimeout(t);
          resolve(JSON.parse(line));
        }
      });
      goProc.on("error", reject);
      goProc.on("exit", (code) =>
        reject(new Error(`go harness exited early: ${code}`)),
      );
    });

    libp2p = await createLibp2p({
      privateKey: identity.privateKey,
      transports: [webSockets()],
      connectionEncrypters: [noise()],
      streamMuxers: [yamux()],
    });
    client = new LinkSelfClient({
      libp2p,
      identity,
      networkStore: new FixedNetworkStore({
        id: "net-1",
        suiteId: "jp.test",
        members: [identity.did, goNode.did],
        memberRoles: {},
      }),
    });
    await client.start();
    client.node.setOnMessage((_did, payload) => {
      plainMessages.push(dec.decode(payload));
    });
    client.groupShare.registerChannel({ name: "visits", groupId: "net-1" });

    await client.node.connectToAddr(goNode.did, multiaddr(goNode.wsAddr));
  }, 90_000);

  afterAll(async () => {
    await libp2p?.stop();
    goProc?.kill("SIGTERM");
  });

  it("Go → TS: a record put on the Go side arrives and applies here", async () => {
    await client.node.sendMessage(goNode.did, enc.encode("go-put"));
    await eventually(async () => plainMessages.includes("put-done"));

    const rec = await eventually(
      async () => await client.groupShare.get("visits", "go-1"),
    );
    expect(dec.decode(rec.body!)).toBe("from-go");
    expect(rec.did).toBe(goNode.did);
    expect(rec.topic).toBe("area/1");
    expect(rec.groupId).toBe("net-1");
  });

  it("TS → Go: a record put here arrives and applies on the Go side", async () => {
    await client.groupShare.put(
      "visits",
      "area/1",
      "js-1",
      enc.encode("from-js"),
    );

    await client.node.sendMessage(goNode.did, enc.encode("check:js-1"));
    const found = await eventually(
      async () => plainMessages.find((m) => m.startsWith("found:")) ?? null,
    );
    expect(found).toBe("found:from-js");
  });
});
