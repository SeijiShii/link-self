/**
 * M3: real SQL against sqlite-wasm through the SqlProxy, plus the MyDB
 * assembly (KV + SQL) with SQL-write mirroring into devicesync — the same
 * wiring the Go client uses (row-readback).
 */
import { describe, expect, it } from "vitest";
import { MemDeviceStorage, ReplicationEngine } from "../src/devicesync.js";
import { MyDB, wireSqlSync } from "../src/mydb.js";
import { SqliteWasmDatabase } from "../src/sqlite.js";
import { OP_INSERT, OP_UPDATE, SqlProxy } from "../src/sqlproxy.js";

const enc = new TextEncoder();
const dec = new TextDecoder();

describe("SqliteWasmDatabase + SqlProxy (real SQL)", () => {
  it("migrates, binds params, queries, and captures the write log", async () => {
    const db = await SqliteWasmDatabase.open();
    const proxy = await SqlProxy.open(db);

    await proxy.migrate([
      { version: 1, sql: "CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT)" },
    ]);
    await proxy.exec("INSERT INTO users (id, name) VALUES (?, ?)", ["u1", "Alice"]);
    await proxy.exec("UPDATE users SET name = ? WHERE id = ?", ["Alice2", "u1"]);

    const rows = await proxy.query("SELECT * FROM users WHERE id = ?", ["u1"]);
    expect(rows).toEqual([{ id: "u1", name: "Alice2" }]);

    expect(proxy.writeLog.map((w) => w.op)).toEqual([OP_INSERT, OP_UPDATE]);

    // Migrations are idempotent against the real _migrations table.
    await proxy.migrate([
      { version: 1, sql: "CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT)" },
      { version: 2, sql: "CREATE TABLE extras (k TEXT)" },
    ]);
    expect((await proxy.query("SELECT version FROM _migrations ORDER BY version")).length).toBe(2);

    await db.close();
  });

  it("queryOrdered preserves column order", async () => {
    const db = await SqliteWasmDatabase.open();
    await db.exec("CREATE TABLE t (zeta TEXT, alpha TEXT)");
    await db.exec("INSERT INTO t VALUES ('z', 'a')");
    const { columns, rows } = await db.queryOrdered("SELECT * FROM t");
    expect(columns).toEqual(["zeta", "alpha"]);
    expect(rows).toEqual([["z", "a"]]);
    await db.close();
  });
});

describe("MyDB (KV + SQL assembly)", () => {
  async function makeMyDB() {
    const engine = new ReplicationEngine({ storage: new MemDeviceStorage(), selfDID: "did:self" });
    const db = await SqliteWasmDatabase.open();
    const proxy = await SqlProxy.open(db);
    wireSqlSync(proxy, engine);
    return { myDB: new MyDB(engine, proxy), engine, db };
  }

  it("KV operations replicate through the engine and dump/restore round-trips", async () => {
    const { myDB } = await makeMyDB();
    await myDB.put("notes", "n1", enc.encode("v1"));
    await myDB.put("tags", "t1", enc.encode("x"));
    expect(dec.decode((await myDB.get("notes", "n1"))!.body!)).toBe("v1");

    const dump = await myDB.dump();
    expect(dump.map((r) => `${r.table}/${r.id}`).sort()).toEqual(["notes/n1", "tags/t1"]);

    const { myDB: other } = await makeMyDB();
    expect(await other.restore(dump)).toBe(2);
    expect(dec.decode((await other.get("tags", "t1"))!.body!)).toBe("x");
    // Older records are skipped on a second restore.
    expect(await other.restore(dump)).toBe(0);

    await myDB.delete("notes", "n1");
    expect(await myDB.get("notes", "n1")).toBeNull();
  });

  it("SQL writes are mirrored into devicesync (row-readback, Go parity)", async () => {
    const { myDB, engine } = await makeMyDB();
    await myDB.migrate([
      { version: 1, sql: "CREATE TABLE places (id TEXT PRIMARY KEY, label TEXT)" },
    ]);
    await myDB.exec("INSERT INTO places (id, label) VALUES (?, ?)", ["p1", "House A"]);

    const mirrored = await engine.list("places");
    expect(mirrored).toHaveLength(1);
    expect(mirrored[0]!.id).toBe("p1");
    expect(JSON.parse(dec.decode(mirrored[0]!.body!))).toEqual({ id: "p1", label: "House A" });

    // _migrations is never mirrored.
    expect(await engine.list("_migrations")).toHaveLength(0);
  });

  it("sync scope defaults to device and is recordable (Phase C parity)", async () => {
    const { myDB } = await makeMyDB();
    expect(myDB.getSyncScope("anything")).toBe("device");
    myDB.setSyncScope("places", "network");
    expect(myDB.getSyncScope("places")).toBe("network");
  });

  it("SQL methods without a backend throw a clear error", async () => {
    const engine = new ReplicationEngine({ storage: new MemDeviceStorage(), selfDID: "did:self" });
    const kvOnly = new MyDB(engine, null);
    await expect(kvOnly.query("SELECT 1")).rejects.toThrow("no SQL backend");
  });
});
