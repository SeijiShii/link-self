/**
 * Transparent data replication between devices sharing the same DID.
 * Apps use DeviceStorage like a local DB; writes are broadcast to peer
 * devices and applied with last-write-wins conflict resolution.
 * Wire-compatible port of core/internal/devicesync (Go).
 */

/** Storage operation type (wire values match the Go iota order). */
export const OP_PUT = 0;
export const OP_DELETE = 1;
export type DeviceSyncOp = typeof OP_PUT | typeof OP_DELETE;

/**
 * A single mutation in the change log. Every local write produces a
 * ChangeEntry that is broadcast to peer devices for replication.
 */
export interface ChangeEntry {
  /** Monotonically increasing sequence per device. */
  seq: number;
  /** Wall-clock milliseconds. */
  timestamp: number;
  /** Logical table (namespace). */
  table: string;
  /** Unique record identifier within the table. */
  recordId: string;
  op: DeviceSyncOp;
  /** Payload (only for OP_PUT). */
  body: Uint8Array | null;
}

/** A stored item returned by list/get operations. */
export interface DeviceRecord {
  id: string;
  table: string;
  body: Uint8Array | null;
  timestamp: number;
}

/** Serialize a ChangeEntry to the Go-compatible JSON wire format. */
export function marshalChangeEntry(e: ChangeEntry): Uint8Array {
  const json = JSON.stringify({
    seq: e.seq,
    timestamp: e.timestamp,
    table: e.table,
    record_id: e.recordId,
    op: e.op,
    body: e.body == null ? null : bytesToBase64(e.body),
  });
  return new TextEncoder().encode(json);
}

/** Parse a ChangeEntry from the JSON wire format. */
export function unmarshalChangeEntry(data: Uint8Array): ChangeEntry {
  const p = JSON.parse(new TextDecoder().decode(data)) as {
    seq?: number;
    timestamp?: number;
    table?: string;
    record_id?: string;
    op?: number;
    body?: string | null;
  };
  return {
    seq: p.seq ?? 0,
    timestamp: p.timestamp ?? 0,
    table: p.table ?? "",
    recordId: p.record_id ?? "",
    op: p.op === OP_DELETE ? OP_DELETE : OP_PUT,
    body: p.body == null ? null : base64ToBytes(p.body),
  };
}

/** Persistence interface for device-local data. */
export interface DeviceStorage {
  /** Store or update a record; returns the assigned sequence number. */
  put(
    table: string,
    id: string,
    body: Uint8Array | null,
    timestamp: number,
  ): Promise<number>;
  /** Null when not found. */
  get(table: string, id: string): Promise<DeviceRecord | null>;
  /** Remove a record; returns the assigned sequence number. */
  delete(table: string, id: string, timestamp: number): Promise<number>;
  list(table: string): Promise<DeviceRecord[]>;
  /** 0 when the record does not exist. */
  getTimestamp(table: string, id: string): Promise<number>;
  appendChange(entry: ChangeEntry): Promise<void>;
  /** All change entries with seq > since, ordered by seq. */
  changesSince(since: number): Promise<ChangeEntry[]>;
  /** Highest sequence number in the change log (0 if empty). */
  latestSeq(): Promise<number>;
  /** Names of all tables that contain at least one record. */
  listTables(): Promise<string[]>;
  /** Lowest sequence number still present in the change log (0 if empty). */
  minSeq(): Promise<number>;
  /** Remove all change entries with seq < minSeq. */
  truncateChangeLog(minSeq: number): Promise<void>;
}

/** The remote peer during a syncWith handshake. */
export interface SyncPeer {
  /** The peer's highest synced sequence number. */
  latestSeq(): Promise<number>;
  /** Deliver incremental change entries to the peer. */
  sendEntries(entries: ChangeEntry[]): Promise<void>;
  /**
   * Deliver a full data dump (fallback when a gap is detected). Optional;
   * if absent and a gap is detected, syncWith throws.
   */
  sendFullDump?: (records: DeviceRecord[]) => Promise<void>;
}

/** Sends a payload to a specific peer device. */
export type DeviceSendFunc = (
  peerId: string,
  payload: Uint8Array,
) => Promise<void>;

/** Returns the list of peer device IDs (same DID, other devices). */
export type PeerProvider = () => Promise<string[]>;

/** ChangeLog pruning configuration. */
export interface RetentionPolicy {
  /** True = prune by age, false = prune by count. */
  timeBased: boolean;
  /** For time-based, in milliseconds (default: 30 days). */
  maxAgeMs?: number;
  /** For count-based (default: 10000). */
  maxCount?: number;
}

const THIRTY_DAYS_MS = 30 * 24 * 60 * 60 * 1000;
const DEFAULT_MAX_COUNT = 10_000;

/** The default policy: time-based, 30 days. */
export function defaultRetentionPolicy(): RetentionPolicy {
  return { timeBased: true, maxAgeMs: THIRTY_DAYS_MS };
}

export interface ReplicationEngineOptions {
  storage: DeviceStorage;
  send?: DeviceSendFunc | null;
  peers?: PeerProvider | null;
  selfDID: string;
  retention?: RetentionPolicy | null;
  /** Clock override for tests; defaults to Date.now. */
  now?: () => number;
}

/**
 * Transparent replication between devices sharing the same DID: wraps a
 * DeviceStorage, broadcasts every local write, and applies incoming
 * changes with last-write-wins.
 */
export class ReplicationEngine {
  private readonly opts: ReplicationEngineOptions;
  private readonly now: () => number;

  constructor(opts: ReplicationEngineOptions) {
    this.opts = opts;
    this.now = opts.now ?? Date.now;
  }

  /** The underlying device storage (exposed like Go's exported field). */
  get storage(): DeviceStorage {
    return this.opts.storage;
  }

  /** Store a record locally and broadcast the change to peer devices. */
  async put(table: string, id: string, body: Uint8Array): Promise<void> {
    const now = this.now();
    const seq = await this.opts.storage.put(table, id, body, now);
    await this.enforceRetention(now);
    await this.broadcast({
      seq,
      timestamp: now,
      table,
      recordId: id,
      op: OP_PUT,
      body,
    });
  }

  async get(table: string, id: string): Promise<DeviceRecord | null> {
    return await this.opts.storage.get(table, id);
  }

  /** Remove a record locally and broadcast the change to peer devices. */
  async delete(table: string, id: string): Promise<void> {
    const now = this.now();
    const seq = await this.opts.storage.delete(table, id, now);
    await this.broadcast({
      seq,
      timestamp: now,
      table,
      recordId: id,
      op: OP_DELETE,
      body: null,
    });
  }

  async list(table: string): Promise<DeviceRecord[]> {
    return await this.opts.storage.list(table);
  }

  /**
   * Apply a change entry received from a peer device with last-write-wins:
   * only entries with a newer timestamp are applied.
   */
  async handleIncoming(entry: ChangeEntry): Promise<void> {
    const existing = await this.opts.storage.getTimestamp(
      entry.table,
      entry.recordId,
    );
    if (entry.timestamp <= existing) {
      return;
    }
    if (entry.op === OP_PUT) {
      await this.opts.storage.put(
        entry.table,
        entry.recordId,
        entry.body,
        entry.timestamp,
      );
    } else {
      await this.opts.storage.delete(
        entry.table,
        entry.recordId,
        entry.timestamp,
      );
    }
  }

  /**
   * Catch-up sync with a peer device: compare sequence numbers and send
   * incremental changes, or fall back to a full dump when a gap is
   * detected (peer's seq < our minSeq).
   */
  async syncWith(peer: SyncPeer): Promise<void> {
    const peerSeq = await peer.latestSeq();
    const ourLatest = await this.opts.storage.latestSeq();

    if (peerSeq >= ourLatest) {
      await peer.sendEntries([]);
      return;
    }

    const ourMin = await this.opts.storage.minSeq();
    if (ourMin > 0 && peerSeq < ourMin) {
      if (peer.sendFullDump == null) {
        throw new Error(
          `changelog gap detected (peer seq ${peerSeq} < min seq ${ourMin}) and no full dump handler`,
        );
      }
      const tables = await this.opts.storage.listTables();
      const all: DeviceRecord[] = [];
      for (const t of tables) {
        all.push(...(await this.opts.storage.list(t)));
      }
      await peer.sendFullDump(all);
      return;
    }

    await peer.sendEntries(await this.opts.storage.changesSince(peerSeq));
  }

  /** Prune old ChangeLog entries based on the retention policy. */
  private async enforceRetention(nowMs: number): Promise<void> {
    const policy = this.opts.retention;
    if (policy == null) {
      return;
    }
    if (policy.timeBased) {
      const maxAge = policy.maxAgeMs || THIRTY_DAYS_MS;
      const cutoff = nowMs - maxAge;
      const entries = await this.opts.storage.changesSince(0);
      let minSeq = 0;
      for (const e of entries) {
        if (e.timestamp >= cutoff) {
          minSeq = e.seq;
          break;
        }
      }
      if (minSeq > 0) {
        await this.opts.storage.truncateChangeLog(minSeq);
      }
    } else {
      const maxCount = policy.maxCount || DEFAULT_MAX_COUNT;
      const latest = await this.opts.storage.latestSeq();
      if (latest === 0) {
        return;
      }
      const minSeq = await this.opts.storage.minSeq();
      if (minSeq === 0) {
        return;
      }
      const total = latest - minSeq + 1;
      if (total > maxCount) {
        await this.opts.storage.truncateChangeLog(latest - maxCount + 1);
      }
    }
  }

  /**
   * Send a change entry to all peer devices. Individual send failures are
   * ignored — store-and-forward covers offline peers.
   */
  private async broadcast(entry: ChangeEntry): Promise<void> {
    if (this.opts.peers == null || this.opts.send == null) {
      return;
    }
    const peers = await this.opts.peers();
    if (peers.length === 0) {
      return;
    }
    const payload = marshalChangeEntry(entry);
    for (const peer of peers) {
      try {
        await this.opts.send(peer, payload);
      } catch {
        // ignore — offline peers are handled by store-and-forward
      }
    }
  }
}

/** In-memory DeviceStorage for tests and validation. */
export class MemDeviceStorage implements DeviceStorage {
  private readonly records = new Map<string, DeviceRecord>();
  private log: ChangeEntry[] = [];
  private seq = 0;

  private key(table: string, id: string): string {
    return `${table} ${id}`;
  }

  async put(
    table: string,
    id: string,
    body: Uint8Array | null,
    timestamp: number,
  ): Promise<number> {
    const seq = ++this.seq;
    const copy = body == null ? null : body.slice();
    this.records.set(this.key(table, id), { id, table, body: copy, timestamp });
    this.log.push({
      seq,
      timestamp,
      table,
      recordId: id,
      op: OP_PUT,
      body: copy,
    });
    return seq;
  }

  async get(table: string, id: string): Promise<DeviceRecord | null> {
    return this.records.get(this.key(table, id)) ?? null;
  }

  async delete(table: string, id: string, timestamp: number): Promise<number> {
    const seq = ++this.seq;
    this.records.delete(this.key(table, id));
    this.log.push({
      seq,
      timestamp,
      table,
      recordId: id,
      op: OP_DELETE,
      body: null,
    });
    return seq;
  }

  async list(table: string): Promise<DeviceRecord[]> {
    return [...this.records.values()].filter((r) => r.table === table);
  }

  async getTimestamp(table: string, id: string): Promise<number> {
    return this.records.get(this.key(table, id))?.timestamp ?? 0;
  }

  async appendChange(entry: ChangeEntry): Promise<void> {
    this.log.push(entry);
    if (entry.seq > this.seq) {
      this.seq = entry.seq;
    }
  }

  async changesSince(since: number): Promise<ChangeEntry[]> {
    return this.log.filter((e) => e.seq > since);
  }

  async latestSeq(): Promise<number> {
    return this.seq;
  }

  async listTables(): Promise<string[]> {
    return [...new Set([...this.records.values()].map((r) => r.table))];
  }

  async minSeq(): Promise<number> {
    return this.log[0]?.seq ?? 0;
  }

  async truncateChangeLog(minSeq: number): Promise<void> {
    this.log = this.log.filter((e) => e.seq >= minSeq);
  }
}

function bytesToBase64(bytes: Uint8Array): string {
  let bin = "";
  for (const b of bytes) {
    bin += String.fromCharCode(b);
  }
  return btoa(bin);
}

function base64ToBytes(b64: string): Uint8Array {
  const bin = atob(b64);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) {
    out[i] = bin.charCodeAt(i);
  }
  return out;
}
