/**
 * Real-browser E2E for @linkself/core (Chromium via Playwright):
 *   1. cross-origin isolation is active (required by the OPFS VFS)
 *   2. browser → Go node over WebSocket: LinkSelf auth + echo round-trip
 *   3. OPFS sqlite persistence across a page reload
 *   4. Web Locks serialize two tabs of the same origin
 *
 * The Go harness (core/cmd/poc-wsnode) is spawned per test run.
 */
import { spawn, type ChildProcess } from "node:child_process";
import { fileURLToPath } from "node:url";
import { expect, test } from "@playwright/test";

const CORE_DIR = fileURLToPath(new URL("../../../core", import.meta.url));

let goProc: ChildProcess;
let goNode: { did: string; wsAddr: string };

test.beforeAll(async () => {
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
  });
});

test.afterAll(() => {
  goProc?.kill("SIGTERM");
});

test("page is cross-origin isolated (OPFS VFS prerequisite)", async ({ page }) => {
  await page.goto("/");
  await page.waitForFunction(() => window.linkselfE2E != null);
  expect(await page.evaluate(() => window.linkselfE2E.crossOriginIsolated)).toBe(true);
});

test("browser connects to the Go node over WebSocket and echoes", async ({ page }) => {
  await page.goto("/");
  await page.waitForFunction(() => window.linkselfE2E != null);
  const reply = await page.evaluate(
    ([addr, did]) => window.linkselfE2E.p2pEcho(addr!, did!),
    [goNode.wsAddr, goNode.did],
  );
  expect(reply).toBe("echo:hello from real browser");
});

test("sqlite data persists in OPFS across a reload", async ({ page }) => {
  await page.goto("/");
  await page.waitForFunction(() => window.linkselfE2E != null);
  expect(await page.evaluate(() => window.linkselfE2E.opfsWrite())).toBeGreaterThan(0);

  await page.reload();
  await page.waitForFunction(() => window.linkselfE2E != null);
  expect(await page.evaluate(() => window.linkselfE2E.opfsRead())).toBe("met");
});

test("Web Locks serialize two tabs", async ({ context }) => {
  const page1 = await context.newPage();
  const page2 = await context.newPage();
  await page1.goto("/");
  await page2.goto("/");
  await page1.waitForFunction(() => window.linkselfE2E != null);
  await page2.waitForFunction(() => window.linkselfE2E != null);

  const start = Date.now();
  const [r1, r2] = await Promise.all([
    page1.evaluate(() => window.linkselfE2E.acquireLock("linkself-db", 800)),
    page2.evaluate(() => window.linkselfE2E.acquireLock("linkself-db", 800)),
  ]);
  const elapsed = Date.now() - start;

  // Both eventually acquired, but not concurrently: total ≥ 2 hold periods.
  expect(r1).toMatch(/^held:/);
  expect(r2).toMatch(/^held:/);
  expect(elapsed).toBeGreaterThanOrEqual(1600);
});
