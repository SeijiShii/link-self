/**
 * SqlDatabase implementation backed by the official sqlite-wasm build
 * (@sqlite.org/sqlite-wasm) — the browser-side counterpart of Go's
 * ncruces/go-sqlite3.
 *
 * - In the browser, pass a filename to persist via OPFS (the module picks
 *   the OPFS VFS when available; requires COOP/COEP headers for the
 *   worker-based VFS or the opfs-sahpool VFS).
 * - In Node (tests) or with no filename, the database is in-memory.
 *
 * Multi-tab note: OPFS SQLite must be accessed by one connection at a
 * time. Serialize access via Web Locks / a SharedWorker at the app layer;
 * this class does not arbitrate between tabs.
 */
import sqlite3InitModule from "@sqlite.org/sqlite-wasm";
import type { SqlDatabase } from "./sqlproxy.js";

type Sqlite3Static = Awaited<ReturnType<typeof sqlite3InitModule>>;

let sqlite3Promise: Promise<Sqlite3Static> | null = null;

/** Initialize (once) and cache the sqlite-wasm module. */
async function sqlite3(): Promise<Sqlite3Static> {
  sqlite3Promise ??= sqlite3InitModule();
  return await sqlite3Promise;
}

export interface SqliteWasmOptions {
  /**
   * Database filename. Empty / ":memory:" = in-memory. In the browser a
   * plain name (e.g. "linkself/data.db") is stored in OPFS when an OPFS
   * VFS is available.
   */
  filename?: string;
}

/** sqlite-wasm implementation of the SqlDatabase abstraction. */
export class SqliteWasmDatabase implements SqlDatabase {
  private constructor(
    // oo1.DB instance — typed loosely; the oo1 API is not fully typed upstream.
    private readonly db: {
      exec(opts: object): unknown;
      close(): void;
    },
  ) {}

  static async open(opts: SqliteWasmOptions = {}): Promise<SqliteWasmDatabase> {
    const mod = await sqlite3();
    const oo1 = (
      mod as unknown as {
        oo1: {
          DB: new (...args: unknown[]) => never;
          OpfsDb?: new (...args: unknown[]) => never;
        };
      }
    ).oo1;
    const filename = opts.filename ?? ":memory:";
    const useOpfs =
      filename !== ":memory:" && filename !== "" && oo1.OpfsDb != null;
    const db = useOpfs
      ? new oo1.OpfsDb!(filename, "c")
      : new oo1.DB(filename, "c");
    return new SqliteWasmDatabase(db);
  }

  async exec(sql: string, params?: unknown[]): Promise<void> {
    this.db.exec({
      sql,
      bind: params != null && params.length > 0 ? params : undefined,
    });
  }

  async query(
    sql: string,
    params?: unknown[],
  ): Promise<Array<Record<string, unknown>>> {
    const resultRows: Array<Record<string, unknown>> = [];
    this.db.exec({
      sql,
      bind: params != null && params.length > 0 ? params : undefined,
      rowMode: "object",
      resultRows,
    });
    return resultRows;
  }

  /**
   * Like query, but preserves column order (needed e.g. to identify the
   * first column as the primary key by convention).
   */
  async queryOrdered(
    sql: string,
    params?: unknown[],
  ): Promise<{ columns: string[]; rows: unknown[][] }> {
    const rows: unknown[][] = [];
    const columns: string[] = [];
    this.db.exec({
      sql,
      bind: params != null && params.length > 0 ? params : undefined,
      rowMode: "array",
      resultRows: rows,
      columnNames: columns,
    });
    return { columns, rows };
  }

  async close(): Promise<void> {
    this.db.close();
  }
}
