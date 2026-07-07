/**
 * SQL query interface that wraps a SQL database and captures write
 * operations for sync. Apps use standard SQL (exec/query); writes are
 * detected and logged for the sync engine to broadcast.
 * Port of core/internal/sqlproxy (Go).
 *
 * The Go version binds database/sql + SQLite directly; here the database
 * is abstracted behind SqlDatabase so the browser build can inject
 * sqlite-wasm (OPFS) in M3 and tests can inject a fake.
 */

/** The type of a detected write operation (values match the Go iota order). */
export const OP_INSERT = 0;
export const OP_UPDATE = 1;
export const OP_SQL_DELETE = 2;
export type WriteOp = typeof OP_INSERT | typeof OP_UPDATE | typeof OP_SQL_DELETE;

/** A detected write operation, recorded for sync. */
export interface WriteEntry {
  op: WriteOp;
  sql: string;
}

/** A structured write event with the target table name extracted. */
export interface WriteEvent {
  table: string;
  op: WriteOp;
  sql: string;
}

/** A schema migration step. */
export interface SqlMigration {
  version: number;
  sql: string;
}

export type OnWriteFunc = (entry: WriteEntry) => void;
export type OnWriteEventFunc = (event: WriteEvent) => Promise<void> | void;

/**
 * Minimal SQL database abstraction. M3 provides a sqlite-wasm (OPFS)
 * implementation; the Go side's equivalent is database/sql + SQLite.
 */
export interface SqlDatabase {
  /** Execute a statement (DDL / write). */
  exec(sql: string, params?: unknown[]): Promise<void>;
  /** Execute a query and return all rows as objects keyed by column name. */
  query(sql: string, params?: unknown[]): Promise<Array<Record<string, unknown>>>;
}

/** Wraps a SQL database and intercepts write operations. */
export class SqlProxy {
  /** Captured writes (for the sync engine to consume). */
  readonly writeLog: WriteEntry[] = [];
  onWrite?: OnWriteFunc;
  onWriteEvent?: OnWriteEventFunc;

  private constructor(private readonly db: SqlDatabase) {}

  /**
   * Create a proxy over the given database and ensure the migration
   * tracking table exists.
   */
  static async open(db: SqlDatabase): Promise<SqlProxy> {
    await db.exec("CREATE TABLE IF NOT EXISTS _migrations (version INTEGER PRIMARY KEY)");
    return new SqlProxy(db);
  }

  /**
   * Execute a SQL statement. Writes (INSERT/UPDATE/DELETE/REPLACE) are
   * recorded in writeLog and reported to the callbacks.
   */
  async exec(sql: string, params?: unknown[]): Promise<void> {
    await this.db.exec(sql, params);
    const op = detectWrite(sql);
    if (op == null) {
      return;
    }
    const entry: WriteEntry = { op, sql };
    this.writeLog.push(entry);
    this.onWrite?.(entry);
    if (this.onWriteEvent != null) {
      await this.onWriteEvent({ table: extractTableName(sql, op), op, sql });
    }
  }

  /** Execute a query that returns rows (SELECT). */
  async query(sql: string, params?: unknown[]): Promise<Array<Record<string, unknown>>> {
    return await this.db.query(sql, params);
  }

  /** Run migrations in order, skipping already-applied versions. */
  async migrate(migrations: SqlMigration[]): Promise<void> {
    for (const m of migrations) {
      const rows = await this.db.query("SELECT COUNT(*) AS n FROM _migrations WHERE version = ?", [
        m.version,
      ]);
      const n = Number(rows[0]?.n ?? 0);
      if (n > 0) {
        continue;
      }
      await this.db.exec(m.sql);
      await this.db.exec("INSERT INTO _migrations (version) VALUES (?)", [m.version]);
    }
  }
}

/** Check whether a SQL statement is a write operation. */
export function detectWrite(sql: string): WriteOp | null {
  const trimmed = sql.trimStart().toUpperCase();
  if (trimmed.startsWith("INSERT")) return OP_INSERT;
  if (trimmed.startsWith("UPDATE")) return OP_UPDATE;
  if (trimmed.startsWith("DELETE")) return OP_SQL_DELETE;
  if (trimmed.startsWith("REPLACE")) return OP_INSERT;
  return null;
}

/**
 * Extract the target table name from a write SQL statement. Handles:
 *   INSERT [OR REPLACE] INTO <table> …, REPLACE INTO <table> …,
 *   UPDATE <table> SET …, DELETE FROM <table> …
 */
export function extractTableName(sql: string, op: WriteOp): string {
  const tokens = sql.split(/\s+/).filter((t) => t !== "");
  switch (op) {
    case OP_INSERT:
      for (let i = 0; i < tokens.length - 1; i++) {
        if (tokens[i]!.toUpperCase() === "INTO") {
          return stripQuotes(tokens[i + 1]!);
        }
      }
      break;
    case OP_UPDATE:
      if (tokens.length >= 2) {
        return stripQuotes(tokens[1]!);
      }
      break;
    case OP_SQL_DELETE:
      for (let i = 0; i < tokens.length - 1; i++) {
        if (tokens[i]!.toUpperCase() === "FROM") {
          return stripQuotes(tokens[i + 1]!);
        }
      }
      break;
  }
  return "";
}

/** Remove surrounding quotes and trailing parentheses from a table name. */
function stripQuotes(s: string): string {
  return s.replace(/\(+$/, "").replace(/^["'`[\]]+|["'`[\]]+$/g, "");
}
