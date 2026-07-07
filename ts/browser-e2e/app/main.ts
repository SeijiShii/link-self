/**
 * Browser-side harness: exposes scenario functions on window.linkselfE2E
 * for the Playwright spec to call via page.evaluate.
 */
import { noise } from "@chainsafe/libp2p-noise";
import { yamux } from "@chainsafe/libp2p-yamux";
import { webSockets } from "@libp2p/websockets";
import {
  generateIdentity,
  LinkSelfClient,
  SqliteWasmDatabase,
  SqlProxy,
} from "@linkself/core";
import { multiaddr } from "@multiformats/multiaddr";
import { createLibp2p } from "libp2p";

const enc = new TextEncoder();
const dec = new TextDecoder();

function log(msg: string): void {
  document.getElementById("log")!.textContent += msg + "\n";
}

/**
 * Scenario: connect to the Go echo node over WebSocket from the browser,
 * run LinkSelf auth, send a message, and return the echo reply.
 */
async function p2pEcho(wsAddr: string, goDID: string): Promise<string> {
  const identity = await generateIdentity();
  const libp2p = await createLibp2p({
    privateKey: identity.privateKey,
    transports: [webSockets()],
    connectionEncrypters: [noise()],
    streamMuxers: [yamux()],
  });
  const client = new LinkSelfClient({ libp2p, identity });
  await client.start();

  const reply = new Promise<string>((resolve, reject) => {
    const t = setTimeout(() => reject(new Error("echo timeout")), 15_000);
    client.node.setOnMessage((_did, payload) => {
      clearTimeout(t);
      resolve(dec.decode(payload));
    });
  });

  log(`dialing ${wsAddr}`);
  await client.node.connectToAddr(goDID, multiaddr(wsAddr));
  log("auth ok, sending message");
  await client.node.sendMessage(goDID, enc.encode("hello from real browser"));
  const result = await reply;
  await libp2p.stop();
  return result;
}

/** Scenario: write through SqlProxy into an OPFS-backed sqlite database. */
async function opfsWrite(): Promise<number> {
  const db = await SqliteWasmDatabase.open({ filename: "e2e-data.db" });
  const proxy = await SqlProxy.open(db);
  await proxy.migrate([
    { version: 1, sql: "CREATE TABLE visits (id TEXT PRIMARY KEY, status TEXT)" },
  ]);
  await proxy.exec("INSERT OR REPLACE INTO visits (id, status) VALUES (?, ?)", ["v1", "met"]);
  const rows = await proxy.query("SELECT COUNT(*) AS n FROM visits");
  await db.close();
  return Number(rows[0]!.n);
}

/** Scenario: read back after a page reload — proves OPFS persistence. */
async function opfsRead(): Promise<string | null> {
  const db = await SqliteWasmDatabase.open({ filename: "e2e-data.db" });
  const rows = await db.query("SELECT status FROM visits WHERE id = ?", ["v1"]);
  await db.close();
  return rows.length > 0 ? String(rows[0]!.status) : null;
}

/** Scenario: report whether this page currently holds the named Web Lock. */
async function acquireLock(name: string, holdMs: number): Promise<string> {
  return await navigator.locks.request(name, async () => {
    await new Promise((r) => setTimeout(r, holdMs));
    return `held:${Date.now()}`;
  });
}

declare global {
  interface Window {
    linkselfE2E: {
      p2pEcho: typeof p2pEcho;
      opfsWrite: typeof opfsWrite;
      opfsRead: typeof opfsRead;
      acquireLock: typeof acquireLock;
      crossOriginIsolated: boolean;
    };
  }
}

window.linkselfE2E = {
  p2pEcho,
  opfsWrite,
  opfsRead,
  acquireLock,
  crossOriginIsolated: globalThis.crossOriginIsolated,
};
log(`ready (crossOriginIsolated=${globalThis.crossOriginIsolated})`);
