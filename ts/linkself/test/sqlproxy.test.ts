import { describe, expect, it } from "vitest";
import {
  detectWrite,
  extractTableName,
  OP_INSERT,
  OP_SQL_DELETE,
  OP_UPDATE,
  SqlProxy,
  type SqlDatabase,
  type WriteEvent,
} from "../src/sqlproxy.js";

describe("detectWrite (mirroring proxy_test.go)", () => {
  it("detects each write kind, case-insensitively and with leading whitespace", () => {
    expect(detectWrite("INSERT INTO t VALUES (1)")).toBe(OP_INSERT);
    expect(detectWrite("  insert into t values (1)")).toBe(OP_INSERT);
    expect(detectWrite("REPLACE INTO t VALUES (1)")).toBe(OP_INSERT);
    expect(detectWrite("UPDATE t SET a = 1")).toBe(OP_UPDATE);
    expect(detectWrite("DELETE FROM t WHERE a = 1")).toBe(OP_SQL_DELETE);
  });

  it("does not flag reads or DDL", () => {
    expect(detectWrite("SELECT * FROM t")).toBeNull();
    expect(detectWrite("CREATE TABLE t (a INTEGER)")).toBeNull();
    expect(detectWrite("PRAGMA journal_mode=WAL")).toBeNull();
  });
});

describe("extractTableName (mirroring proxy_test.go)", () => {
  it("handles INSERT variants", () => {
    expect(extractTableName("INSERT INTO users VALUES (1)", OP_INSERT)).toBe(
      "users",
    );
    expect(
      extractTableName("INSERT OR REPLACE INTO users VALUES (1)", OP_INSERT),
    ).toBe("users");
    expect(extractTableName("REPLACE INTO users VALUES (1)", OP_INSERT)).toBe(
      "users",
    );
    expect(
      extractTableName('INSERT INTO "users" (id) VALUES (1)', OP_INSERT),
    ).toBe("users");
    // Parity with Go: stripQuotes only trims TRAILING "(", so a table token
    // fused with a column list is returned as-is (same limitation as Go).
    expect(
      extractTableName("INSERT INTO users(id) VALUES (1)", OP_INSERT),
    ).toBe("users(id)");
    expect(extractTableName("INSERT INTO users( VALUES (1)", OP_INSERT)).toBe(
      "users",
    );
  });

  it("handles UPDATE and DELETE", () => {
    expect(extractTableName("UPDATE users SET name = 'x'", OP_UPDATE)).toBe(
      "users",
    );
    expect(extractTableName("UPDATE `users` SET name = 'x'", OP_UPDATE)).toBe(
      "users",
    );
    expect(
      extractTableName("DELETE FROM users WHERE id = 1", OP_SQL_DELETE),
    ).toBe("users");
    expect(extractTableName("DELETE FROM [users]", OP_SQL_DELETE)).toBe(
      "users",
    );
  });
});

/** Scripted fake database: records executed SQL, simulates _migrations. */
class FakeDB implements SqlDatabase {
  executed: string[] = [];
  applied = new Set<number>();

  async exec(sql: string, params?: unknown[]): Promise<void> {
    this.executed.push(sql);
    if (sql.startsWith("INSERT INTO _migrations")) {
      this.applied.add(Number(params?.[0]));
    }
  }

  async query(
    sql: string,
    params?: unknown[],
  ): Promise<Array<Record<string, unknown>>> {
    if (sql.includes("FROM _migrations")) {
      return [{ n: this.applied.has(Number(params?.[0])) ? 1 : 0 }];
    }
    return [];
  }
}

describe("SqlProxy", () => {
  it("logs writes and reports structured events; reads are not logged", async () => {
    const db = new FakeDB();
    const proxy = await SqlProxy.open(db);
    const events: WriteEvent[] = [];
    proxy.onWriteEvent = (e) => {
      events.push(e);
    };

    await proxy.exec("INSERT INTO users (id) VALUES (?)", [1]);
    await proxy.exec("UPDATE users SET name = 'a'");
    await proxy.query("SELECT * FROM users");
    await proxy.exec("CREATE INDEX idx ON users(id)");

    expect(proxy.writeLog.map((w) => w.op)).toEqual([OP_INSERT, OP_UPDATE]);
    expect(events.map((e) => e.table)).toEqual(["users", "users"]);
  });

  it("migrate applies pending migrations once and skips applied ones", async () => {
    const db = new FakeDB();
    const proxy = await SqlProxy.open(db);
    const migrations = [
      { version: 1, sql: "CREATE TABLE a (x)" },
      { version: 2, sql: "CREATE TABLE b (y)" },
    ];
    await proxy.migrate(migrations);
    expect(db.executed).toContain("CREATE TABLE a (x)");
    expect(db.executed).toContain("CREATE TABLE b (y)");
    expect(db.applied).toEqual(new Set([1, 2]));

    const before = db.executed.length;
    await proxy.migrate(migrations); // second run: all skipped
    expect(db.executed.length).toBe(before);
  });

  it("migration DDL is not recorded in the write log", async () => {
    const db = new FakeDB();
    const proxy = await SqlProxy.open(db);
    await proxy.migrate([{ version: 1, sql: "CREATE TABLE a (x)" }]);
    expect(proxy.writeLog).toHaveLength(0);
  });
});
