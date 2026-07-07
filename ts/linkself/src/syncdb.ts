/**
 * Sync layer: meta attachment, immediate group delivery, last-write-wins
 * apply. Wire-compatible port of core/internal/syncdb (Go).
 * See link-self docs/spec/sync-db-plan.md.
 */

/** The unit of sync: meta plus app payload. */
export interface SyncRecord {
  /** Unique; used for conflict resolution. */
  id: string;
  /** Target group for delivery. */
  groupId: string;
  /** Writer's DID. */
  did: string;
  /** Milliseconds; for last-write-wins. */
  timestamp: number;
  /** App payload (e.g. JSON). Null matches Go's nil slice. */
  body: Uint8Array | null;
  /** If true, treat as delete (apply as Delete on remote). */
  deleted: boolean;
}

/** Serialize a record to the Go-compatible JSON wire format. */
export function marshalRecord(record: SyncRecord): Uint8Array {
  const json = JSON.stringify({
    id: record.id,
    groupId: record.groupId,
    did: record.did,
    timestamp: record.timestamp,
    body: record.body == null ? null : bytesToBase64(record.body),
    deleted: record.deleted,
  });
  return new TextEncoder().encode(json);
}

/** Parse a record from the JSON wire format. Throws on invalid input. */
export function unmarshalRecord(data: Uint8Array): SyncRecord {
  const parsed = JSON.parse(new TextDecoder().decode(data)) as {
    id?: string;
    groupId?: string;
    did?: string;
    timestamp?: number;
    body?: string | null;
    deleted?: boolean;
  };
  return {
    id: parsed.id ?? "",
    groupId: parsed.groupId ?? "",
    did: parsed.did ?? "",
    timestamp: parsed.timestamp ?? 0,
    body: parsed.body == null ? null : base64ToBytes(parsed.body),
    deleted: parsed.deleted ?? false,
  };
}

/** The storage interface the sync layer depends on. App injects an implementation. */
export interface RecordStorage {
  put(record: SyncRecord): Promise<void>;
  /** Returns null when not found. */
  get(id: string): Promise<SyncRecord | null>;
  /** Returns 0 when not found (apply allowed). */
  getTimestamp(id: string): Promise<number>;
  delete(id: string): Promise<void>;
  list(): Promise<SyncRecord[]>;
}

/** Returns member DIDs for a group (recipients for delivery). */
export interface MemberResolver {
  memberDIDsForGroup(groupId: string): Promise<string[]>;
}

/** Sends a payload to the given member DIDs (e.g. node.sendToGroup). */
export type SendGroupFunc = (memberDIDs: string[], payload: Uint8Array) => Promise<void>;

/**
 * Applies meta, persists, delivers to group, and applies incoming records
 * with last-write-wins.
 */
export class SyncLayer {
  constructor(
    readonly storage: RecordStorage,
    private readonly resolver: MemberResolver,
    private readonly sendGroup: SendGroupFunc,
    private readonly selfDID: string,
  ) {}

  /**
   * Persist the record with meta (id, did, timestamp), then send to group
   * members. Delivery failures are ignored (matching Go — offline members
   * are covered by store-and-forward).
   */
  async put(groupId: string, body: Uint8Array): Promise<SyncRecord> {
    const record: SyncRecord = {
      id: crypto.randomUUID(),
      groupId,
      did: this.selfDID,
      timestamp: Date.now(),
      body,
      deleted: false,
    };
    await this.storage.put(record);
    const payload = marshalRecord(record);
    const memberDIDs = await this.resolver.memberDIDsForGroup(groupId);
    await this.sendGroup(memberDIDs, payload).catch(() => {});
    return record;
  }

  /** Persist an existing record (e.g. with pre-set id), without delivery. */
  async putRecord(record: SyncRecord): Promise<void> {
    await this.storage.put(record);
  }

  /**
   * Decode the payload as a SyncRecord and apply with last-write-wins:
   * apply only if the incoming timestamp is newer than the stored one.
   */
  async handleIncoming(payload: Uint8Array): Promise<void> {
    const record = unmarshalRecord(payload);
    const existing = await this.storage.getTimestamp(record.id);
    if (record.timestamp <= existing) {
      return;
    }
    if (record.deleted) {
      await this.storage.delete(record.id);
      return;
    }
    await this.storage.put(record);
  }
}

/** In-memory RecordStorage for tests and validation. */
export class MemStorage implements RecordStorage {
  private readonly byId = new Map<string, SyncRecord>();

  async put(record: SyncRecord): Promise<void> {
    this.byId.set(record.id, copyRecord(record));
  }

  async get(id: string): Promise<SyncRecord | null> {
    const r = this.byId.get(id);
    return r == null ? null : copyRecord(r);
  }

  async getTimestamp(id: string): Promise<number> {
    return this.byId.get(id)?.timestamp ?? 0;
  }

  async delete(id: string): Promise<void> {
    this.byId.delete(id);
  }

  async list(): Promise<SyncRecord[]> {
    return Array.from(this.byId.values(), copyRecord);
  }
}

function copyRecord(r: SyncRecord): SyncRecord {
  return { ...r, body: r.body == null ? null : r.body.slice() };
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
