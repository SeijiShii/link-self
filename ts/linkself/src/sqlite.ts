/**
 * SqlDatabase implementation backed by the official sqlite-wasm build
 * (@sqlite.org/sqlite-wasm) — the browser-side counterpart of Go's
 * ncruces/go-sqlite3.
 *
 * - Persistent (a filename): runs sqlite in a dedicated Worker using the
 *   **OPFS SAHPool VFS**. That VFS needs `createSyncAccessHandle`, which
 *   browsers expose only in workers, so it cannot run on the main thread.
 *   Requires cross-origin isolation (COOP/COEP). Verified persisting across
 *   reloads by ts/browser-e2e.
 * - In-memory (":memory:" / no filename): runs on the current thread via
 *   `oo1.DB`. Used by Node tests and for ephemeral data.
 *
 * Multi-tab note: the SAHPool VFS acquires exclusive sync access handles,
 * so only one worker (hence one tab) may hold a given database at a time.
 * Serialize access across tabs via Web Locks / a SharedWorker at the app
 * layer; this class does not arbitrate between tabs.
 */
import sqlite3InitModule from "@sqlite.org/sqlite-wasm";
import type { SqlDatabase } from "./sqlproxy.js";

/** Minimal shape of an oo1-style DB handle (oo1 is loosely typed upstream). */
interface Sqlite3DbHandle {
  exec(opts: object): unknown;
  close(): void;
}

type Sqlite3Static = Awaited<ReturnType<typeof sqlite3InitModule>>;

let sqlite3Promise: Promise<Sqlite3Static> | null = null;

/** Initialize (once) and cache the sqlite-wasm module (main thread). */
async function sqlite3(): Promise<Sqlite3Static> {
  sqlite3Promise ??= sqlite3InitModule();
  return await sqlite3Promise;
}

/** The 4 async operations SqliteWasmDatabase delegates to a backend. */
interface DbBackend {
  exec(sql: string, params?: unknown[]): Promise<void>;
  query(sql: string, params?: unknown[]): Promise<Array<Record<string, unknown>>>;
  queryOrdered(
    sql: string,
    params?: unknown[],
  ): Promise<{ columns: string[]; rows: unknown[][] }>;
  close(): Promise<void>;
}

function bindOf(params?: unknown[]): unknown[] | undefined {
  return params != null && params.length > 0 ? params : undefined;
}

/** Main-thread backend over a synchronous oo1 DB handle (in-memory). */
class MainThreadBackend implements DbBackend {
  constructor(private readonly db: Sqlite3DbHandle) {}

  async exec(sql: string, params?: unknown[]): Promise<void> {
    this.db.exec({ sql, bind: bindOf(params) });
  }

  async query(sql: string, params?: unknown[]): Promise<Array<Record<string, unknown>>> {
    const resultRows: Array<Record<string, unknown>> = [];
    this.db.exec({ sql, bind: bindOf(params), rowMode: "object", resultRows });
    return resultRows;
  }

  async queryOrdered(
    sql: string,
    params?: unknown[],
  ): Promise<{ columns: string[]; rows: unknown[][] }> {
    const rows: unknown[][] = [];
    const columns: string[] = [];
    this.db.exec({ sql, bind: bindOf(params), rowMode: "array", resultRows: rows, columnNames: columns });
    return { columns, rows };
  }

  async close(): Promise<void> {
    this.db.close();
  }
}

interface RpcResponse {
  id: number;
  result?: unknown;
  error?: string;
}

/** Worker-RPC backend: sqlite + OPFS SAHPool VFS run in a dedicated worker. */
class WorkerBackend implements DbBackend {
  private seq = 0;
  private readonly pending = new Map<
    number,
    { resolve: (v: unknown) => void; reject: (e: Error) => void }
  >();

  private constructor(private readonly worker: Worker) {
    worker.onmessage = (e: MessageEvent) => {
      const { id, result, error } = e.data as RpcResponse;
      const p = this.pending.get(id);
      if (p == null) return;
      this.pending.delete(id);
      if (error != null) p.reject(new Error(error));
      else p.resolve(result);
    };
  }

  static async open(filename: string): Promise<WorkerBackend> {
    const worker = new Worker(new URL("./sqlite-worker.js", import.meta.url), {
      type: "module",
    });
    const backend = new WorkerBackend(worker);
    await backend.rpc("open", { filename });
    return backend;
  }

  private rpc(method: string, extra: Record<string, unknown>): Promise<unknown> {
    const id = ++this.seq;
    return new Promise<unknown>((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.worker.postMessage({ id, method, ...extra });
    });
  }

  async exec(sql: string, params?: unknown[]): Promise<void> {
    await this.rpc("exec", { sql, params });
  }

  async query(sql: string, params?: unknown[]): Promise<Array<Record<string, unknown>>> {
    return (await this.rpc("query", { sql, params })) as Array<Record<string, unknown>>;
  }

  async queryOrdered(
    sql: string,
    params?: unknown[],
  ): Promise<{ columns: string[]; rows: unknown[][] }> {
    return (await this.rpc("queryOrdered", { sql, params })) as {
      columns: string[];
      rows: unknown[][];
    };
  }

  async close(): Promise<void> {
    await this.rpc("close", {});
    this.worker.terminate();
  }
}

export interface SqliteWasmOptions {
  /**
   * Database filename. Empty / ":memory:" = in-memory (main thread). In the
   * browser a plain name (e.g. "data.db") is persisted in OPFS via a worker.
   */
  filename?: string;
}

/** sqlite-wasm implementation of the SqlDatabase abstraction. */
export class SqliteWasmDatabase implements SqlDatabase {
  private constructor(private readonly backend: DbBackend) {}

  static async open(opts: SqliteWasmOptions = {}): Promise<SqliteWasmDatabase> {
    const filename = opts.filename ?? ":memory:";
    const persistent = filename !== ":memory:" && filename !== "";
    if (persistent) {
      return new SqliteWasmDatabase(await WorkerBackend.open(filename));
    }
    const mod = await sqlite3();
    const oo1 = (mod as unknown as { oo1: { DB: new (...args: unknown[]) => Sqlite3DbHandle } }).oo1;
    return new SqliteWasmDatabase(new MainThreadBackend(new oo1.DB(filename, "c")));
  }

  async exec(sql: string, params?: unknown[]): Promise<void> {
    await this.backend.exec(sql, params);
  }

  async query(sql: string, params?: unknown[]): Promise<Array<Record<string, unknown>>> {
    return await this.backend.query(sql, params);
  }

  /**
   * Like query, but preserves column order (needed e.g. to identify the
   * first column as the primary key by convention).
   */
  async queryOrdered(
    sql: string,
    params?: unknown[],
  ): Promise<{ columns: string[]; rows: unknown[][] }> {
    return await this.backend.queryOrdered(sql, params);
  }

  async close(): Promise<void> {
    await this.backend.close();
  }
}
