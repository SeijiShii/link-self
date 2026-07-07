/**
 * Dedicated worker running sqlite-wasm with the OPFS SAHPool VFS.
 *
 * The SAHPool VFS needs `FileSystemFileHandle.createSyncAccessHandle`, which
 * browsers expose only in dedicated workers — never on the main thread. So a
 * persistent OPFS database must run here. Verified by ts/browser-e2e.
 *
 * RPC: the main thread posts { id, method, sql?, params?, filename? };
 * this worker replies { id, result } or { id, error }.
 */
import sqlite3InitModule from "@sqlite.org/sqlite-wasm";

interface DbHandle {
  exec(opts: object): unknown;
  close(): void;
}

let db: DbHandle | null = null;

async function open(filename: string): Promise<void> {
  const mod = await sqlite3InitModule();
  const pool = await (
    mod as unknown as {
      installOpfsSAHPoolVfs(opts: object): Promise<{
        OpfsSAHPoolDb: new (f: string) => DbHandle;
      }>;
    }
  ).installOpfsSAHPoolVfs({});
  db = new pool.OpfsSAHPoolDb(filename);
}

function bindOf(params?: unknown[]): unknown[] | undefined {
  return params != null && params.length > 0 ? params : undefined;
}

interface RpcRequest {
  id: number;
  method: "open" | "exec" | "query" | "queryOrdered" | "close";
  sql?: string;
  params?: unknown[];
  filename?: string;
}

const ctx = self as unknown as {
  onmessage: ((e: MessageEvent) => void) | null;
  postMessage: (m: unknown) => void;
};

ctx.onmessage = (e: MessageEvent) => {
  const req = e.data as RpcRequest;
  void handle(req)
    .then((result) => ctx.postMessage({ id: req.id, result }))
    .catch((err: unknown) =>
      ctx.postMessage({
        id: req.id,
        error: err instanceof Error ? err.message : String(err),
      }),
    );
};

async function handle(req: RpcRequest): Promise<unknown> {
  if (req.method === "open") {
    await open(req.filename!);
    return null;
  }
  if (db == null) {
    throw new Error("database not open");
  }
  switch (req.method) {
    case "exec":
      db.exec({ sql: req.sql, bind: bindOf(req.params) });
      return null;
    case "query": {
      const resultRows: Array<Record<string, unknown>> = [];
      db.exec({ sql: req.sql, bind: bindOf(req.params), rowMode: "object", resultRows });
      return resultRows;
    }
    case "queryOrdered": {
      const rows: unknown[][] = [];
      const columns: string[] = [];
      db.exec({
        sql: req.sql,
        bind: bindOf(req.params),
        rowMode: "array",
        resultRows: rows,
        columnNames: columns,
      });
      return { columns, rows };
    }
    case "close":
      db.close();
      db = null;
      return null;
  }
}
