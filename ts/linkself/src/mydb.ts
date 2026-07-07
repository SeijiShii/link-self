/**
 * MyDB: the unified data API (KV + SQL). KV operations replicate through
 * devicesync; SQL operations go through the sqlproxy, whose detected
 * writes are mirrored into devicesync (row-readback, matching the Go
 * client's wiring). Port of pkg/linkself myDB (Go).
 */
import type { DeviceRecord, ReplicationEngine } from "./devicesync.js";
import {
  OP_INSERT,
  OP_UPDATE,
  type SqlMigration,
  type SqlProxy,
} from "./sqlproxy.js";

/** Sync scope for a table (Phase C in Go; stored but not yet enforced). */
export type SyncScope = "device" | "network";

export class MyDB {
  private readonly scopeMap = new Map<string, SyncScope>();

  constructor(
    private readonly engine: ReplicationEngine,
    private readonly proxy: SqlProxy | null = null,
  ) {}

  /* ---- KV (replicated via devicesync) ---- */

  async put(table: string, recordId: string, body: Uint8Array): Promise<void> {
    await this.engine.put(table, recordId, body);
  }

  async get(table: string, recordId: string): Promise<DeviceRecord | null> {
    return await this.engine.get(table, recordId);
  }

  async delete(table: string, recordId: string): Promise<void> {
    await this.engine.delete(table, recordId);
  }

  async list(table: string): Promise<DeviceRecord[]> {
    return await this.engine.list(table);
  }

  /** All records across all tables. */
  async dump(): Promise<DeviceRecord[]> {
    const out: DeviceRecord[] = [];
    for (const table of await this.engine.storage.listTables()) {
      out.push(...(await this.engine.list(table)));
    }
    return out;
  }

  /** Apply records with last-write-wins; returns the applied count. */
  async restore(records: DeviceRecord[]): Promise<number> {
    let applied = 0;
    const storage = this.engine.storage;
    for (const r of records) {
      const existing = await storage.getTimestamp(r.table, r.id);
      if (r.timestamp > existing) {
        await storage.put(r.table, r.id, r.body, r.timestamp);
        applied++;
      }
    }
    return applied;
  }

  /* ---- SQL (requires a SqlDatabase backend, e.g. sqlite-wasm) ---- */

  private mustProxy(): SqlProxy {
    if (this.proxy == null) {
      throw new Error(
        "MyDB: no SQL backend configured (pass sqlDatabase to the client)",
      );
    }
    return this.proxy;
  }

  async exec(sql: string, params?: unknown[]): Promise<void> {
    await this.mustProxy().exec(sql, params);
  }

  async query(
    sql: string,
    params?: unknown[],
  ): Promise<Array<Record<string, unknown>>> {
    return await this.mustProxy().query(sql, params);
  }

  async migrate(migrations: SqlMigration[]): Promise<void> {
    await this.mustProxy().migrate(migrations);
  }

  /** Record the sync scope for a table (enforcement lands with Phase C, as in Go). */
  setSyncScope(table: string, scope: SyncScope): void {
    this.scopeMap.set(table, scope);
  }

  /** The sync scope for a table (default: "device", matching Go). */
  getSyncScope(table: string): SyncScope {
    return this.scopeMap.get(table) ?? "device";
  }
}

/**
 * Mirror detected SQL writes into devicesync, matching the Go client:
 * on INSERT/UPDATE read back all rows of the affected table, serialize
 * each row as JSON, and put it under its first column value (PRIMARY KEY
 * by convention). DELETE mirroring is skipped (same limitation as Go
 * until PK extraction exists). "_migrations" and unknown tables are
 * ignored.
 */
export function wireSqlSync(proxy: SqlProxy, engine: ReplicationEngine): void {
  proxy.onWriteEvent = async (event) => {
    if (event.table === "" || event.table === "_migrations") {
      return;
    }
    if (event.op !== OP_INSERT && event.op !== OP_UPDATE) {
      return;
    }
    let rows: Array<Record<string, unknown>>;
    try {
      rows = await proxy.query(`SELECT * FROM "${event.table}"`);
    } catch {
      return; // best-effort, matching Go
    }
    for (const row of rows) {
      const entries = Object.entries(row);
      if (entries.length === 0) {
        continue;
      }
      const recordId = String(entries[0]![1]);
      const body = new TextEncoder().encode(JSON.stringify(row));
      await engine.put(event.table, recordId, body);
    }
  };
}
