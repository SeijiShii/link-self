/**
 * API-like data sharing between different DIDs in a group. Apps define
 * Channels (named data streams) with schemas and access policies; only data
 * written through a Channel is shared with group members, following
 * app-defined permissions and topic subscriptions.
 * Wire-compatible port of core/internal/groupshare (Go).
 */
import { check as permissionCheck, type Permissions } from "./permission.js";
import type { RoleDAG } from "./role.js";

/* ------------------------------------------------------------------ */
/* Types                                                                */
/* ------------------------------------------------------------------ */

/** Validates a record body before accepting it. Throws on invalid input. */
export interface SchemaValidator {
  validate(body: Uint8Array | null): void;
}

/** Controls who can read/write a channel. */
export interface AccessPolicy {
  canWrite(did: string): boolean;
  canRead(did: string): boolean;
}

/** A named data stream shared within a group. */
export interface Channel {
  name: string;
  groupId: string;
  /** Null = accept any body. */
  schema?: SchemaValidator | null;
  /** Null = allow all. */
  access?: AccessPolicy | null;
  /** Retention in milliseconds. 0 = permanent (master data). */
  retentionMs?: number;
  /** Null = allow all (role-based check). */
  perms?: Permissions | null;
}

/** The unit of data exchanged between group members. */
export interface SharedRecord {
  id: string;
  channel: string;
  /** Topic for subscription filtering ("" = none; omitted on the wire). */
  topic: string;
  groupId: string;
  /** Writer's DID. */
  did: string;
  /** Milliseconds. */
  timestamp: number;
  body: Uint8Array | null;
  deleted: boolean;
}

/** The message sent to peers to announce subscription changes. */
export interface SubAnnouncement {
  did: string;
  channel: string;
  topics: string[];
}

export type GroupShareErrorCode =
  | "channel_exists"
  | "channel_not_found"
  | "access_denied"
  | "schema_validation"
  | "sub_announcement_did_mismatch";

const GROUP_SHARE_ERROR_MESSAGES: Record<GroupShareErrorCode, string> = {
  channel_exists: "channel already registered",
  channel_not_found: "channel not registered",
  access_denied: "access denied",
  schema_validation: "schema validation failed",
  sub_announcement_did_mismatch: "subscription announcement DID mismatch",
};

export class GroupShareError extends Error {
  constructor(readonly code: GroupShareErrorCode) {
    super(GROUP_SHARE_ERROR_MESSAGES[code]);
    this.name = "GroupShareError";
  }
}

/* ------------------------------------------------------------------ */
/* Wire format                                                          */
/* ------------------------------------------------------------------ */

/** Serialize to the Go-compatible JSON wire format (topic omitted when empty). */
export function marshalSharedRecord(rec: SharedRecord): Uint8Array {
  const obj: Record<string, unknown> = { id: rec.id, channel: rec.channel };
  if (rec.topic !== "") {
    obj.topic = rec.topic;
  }
  obj.group_id = rec.groupId;
  obj.did = rec.did;
  obj.timestamp = rec.timestamp;
  obj.body = rec.body == null ? null : bytesToBase64(rec.body);
  obj.deleted = rec.deleted;
  return new TextEncoder().encode(JSON.stringify(obj));
}

/** Parse from the JSON wire format. Throws on invalid JSON. */
export function unmarshalSharedRecord(data: Uint8Array): SharedRecord {
  const p = JSON.parse(new TextDecoder().decode(data)) as {
    id?: string;
    channel?: string;
    topic?: string;
    group_id?: string;
    did?: string;
    timestamp?: number;
    body?: string | null;
    deleted?: boolean;
  };
  return {
    id: p.id ?? "",
    channel: p.channel ?? "",
    topic: p.topic ?? "",
    groupId: p.group_id ?? "",
    did: p.did ?? "",
    timestamp: p.timestamp ?? 0,
    body: p.body == null ? null : base64ToBytes(p.body),
    deleted: p.deleted ?? false,
  };
}

export function marshalSubAnnouncement(ann: SubAnnouncement): Uint8Array {
  return new TextEncoder().encode(
    JSON.stringify({ did: ann.did, channel: ann.channel, topics: ann.topics }),
  );
}

export function unmarshalSubAnnouncement(data: Uint8Array): SubAnnouncement {
  const p = JSON.parse(new TextDecoder().decode(data)) as {
    did?: string;
    channel?: string;
    topics?: string[];
  };
  return { did: p.did ?? "", channel: p.channel ?? "", topics: p.topics ?? [] };
}

/* ------------------------------------------------------------------ */
/* Storage / resolver interfaces                                        */
/* ------------------------------------------------------------------ */

/** Persistence interface for shared records. */
export interface SharedStorage {
  putShared(record: SharedRecord): Promise<void>;
  getShared(channel: string, id: string): Promise<SharedRecord | null>;
  /** 0 when not found (apply allowed). */
  getTimestamp(channel: string, id: string): Promise<number>;
  deleteShared(channel: string, id: string): Promise<void>;
  listByChannel(channel: string): Promise<SharedRecord[]>;
  listByGroup(groupId: string): Promise<SharedRecord[]>;
  listByChannelAndTopic(
    channel: string,
    topic: string,
  ): Promise<SharedRecord[]>;
  /** Remove records in the channel with timestamp < before; returns the count. */
  deleteExpired(channel: string, before: number): Promise<number>;
}

/** Returns the member DIDs for a group, excluding self. */
export interface MemberResolver {
  memberDIDsForGroup(groupId: string): Promise<string[]>;
}

/** Returns the role of a member in a group ("" = no role assigned). */
export interface MemberRoleResolver {
  memberRole(groupId: string, memberDID: string): Promise<string>;
}

/** Sends a payload to a list of DIDs. */
export type SendGroupFunc = (
  memberDIDs: string[],
  payload: Uint8Array,
) => Promise<void>;

/** Stores topic subscriptions per DID per channel. */
export interface SubscriptionStore {
  setSubscription(
    did: string,
    channel: string,
    topics: string[],
  ): Promise<void>;
  /** Null when no subscription is recorded (distinct from []). */
  getSubscription(did: string, channel: string): Promise<string[] | null>;
  getAllSubscriptions(did: string): Promise<Map<string, string[]>>;
}

/**
 * True if the record's topic is covered by the subscribed topics list.
 * Wildcard "*" matches any topic; null/empty means not subscribed.
 */
export function topicMatches(subscribed: string[], topic: string): boolean {
  return subscribed.some((s) => s === "*" || s === topic);
}

/* ------------------------------------------------------------------ */
/* Layer                                                                */
/* ------------------------------------------------------------------ */

export interface GroupShareLayerOptions {
  storage: SharedStorage;
  memberResolver?: MemberResolver | null;
  sendGroup?: SendGroupFunc | null;
  selfDID: string;
  /** This device's subscriptions; null = no persistence. */
  localSubs?: SubscriptionStore | null;
  /** Remote peers' subscriptions; null = send to all. */
  remoteSubs?: SubscriptionStore | null;
  /** Sends sub announcements; null = no broadcast. */
  sendSubAnnounce?: SendGroupFunc | null;
  /** Null = skip role-based permission checks. */
  roleDAG?: RoleDAG | null;
  /** Null = skip role-based permission checks. */
  memberRoleResolver?: MemberRoleResolver | null;
  /** Clock override for tests; defaults to Date.now. */
  now?: () => number;
}

/**
 * Manages app-defined shared data channels and handles sending/receiving
 * shared records between group members.
 */
export class GroupShareLayer {
  private readonly channels = new Map<string, Channel>();
  private readonly opts: GroupShareLayerOptions;
  private readonly now: () => number;

  constructor(opts: GroupShareLayerOptions) {
    this.opts = opts;
    this.now = opts.now ?? Date.now;
  }

  /** Register a named data channel for sharing within a group. */
  registerChannel(ch: Channel): void {
    if (this.channels.has(ch.name)) {
      throw new GroupShareError("channel_exists");
    }
    this.channels.set(ch.name, ch);
  }

  /**
   * Declare which topics this device wants for a channel. ["*"] = all
   * topics, [] = unsubscribe. Stores locally and broadcasts a
   * SubAnnouncement to group members.
   */
  async subscribe(channel: string, topics: string[]): Promise<void> {
    const ch = this.channels.get(channel);
    if (ch == null) {
      throw new GroupShareError("channel_not_found");
    }
    if (this.opts.localSubs != null) {
      await this.opts.localSubs.setSubscription(
        this.opts.selfDID,
        channel,
        topics,
      );
    }
    await this.broadcastSubAnnouncement(ch, channel, topics);
  }

  /**
   * Process a subscription announcement from a peer. Validates that
   * senderDID matches the announcement DID to prevent spoofing.
   */
  async handleSubAnnouncement(
    senderDID: string,
    payload: Uint8Array,
  ): Promise<void> {
    const ann = unmarshalSubAnnouncement(payload);
    if (ann.did !== senderDID) {
      throw new GroupShareError("sub_announcement_did_mismatch");
    }
    if (this.opts.remoteSubs != null) {
      await this.opts.remoteSubs.setSubscription(
        ann.did,
        ann.channel,
        ann.topics,
      );
    }
  }

  /**
   * Re-broadcast all local subscriptions to group members. Called after
   * peer reconnection to restore RemoteSubs state on the peer side.
   */
  async announceAllSubscriptions(): Promise<void> {
    if (this.opts.localSubs == null) {
      return;
    }
    const subs = await this.opts.localSubs.getAllSubscriptions(
      this.opts.selfDID,
    );
    for (const [channel, topics] of subs) {
      const ch = this.channels.get(channel);
      if (ch == null) {
        continue;
      }
      await this.broadcastSubAnnouncement(ch, channel, topics);
    }
  }

  /**
   * True if the record has exceeded the channel's retention period. False
   * if the channel has no retention (permanent) or is unknown.
   */
  isExpired(rec: SharedRecord, now: number): boolean {
    const ch = this.channels.get(rec.channel);
    if (ch == null || (ch.retentionMs ?? 0) === 0) {
      return false;
    }
    return now >= rec.timestamp + (ch.retentionMs ?? 0);
  }

  /** Write a shared record to a channel and broadcast to group members. */
  async put(
    channel: string,
    topic: string,
    id: string,
    body: Uint8Array,
  ): Promise<void> {
    const ch = this.channels.get(channel);
    if (ch == null) {
      throw new GroupShareError("channel_not_found");
    }
    if (ch.access != null && !ch.access.canWrite(this.opts.selfDID)) {
      throw new GroupShareError("access_denied");
    }
    await this.checkPerm(ch, this.opts.selfDID, "write");

    const rec: SharedRecord = {
      id,
      channel,
      topic,
      groupId: ch.groupId,
      did: this.opts.selfDID,
      timestamp: this.now(),
      body,
      deleted: false,
    };
    await this.opts.storage.putShared(rec);
    await this.broadcast(ch, rec);
  }

  /** Retrieve a shared record. Returns null if missing or expired. */
  async get(channel: string, id: string): Promise<SharedRecord | null> {
    const rec = await this.opts.storage.getShared(channel, id);
    if (rec == null) {
      return null;
    }
    return this.isExpired(rec, this.now()) ? null : rec;
  }

  /** Mark a shared record as deleted and broadcast to group members. */
  async delete(channel: string, topic: string, id: string): Promise<void> {
    const ch = this.channels.get(channel);
    if (ch == null) {
      throw new GroupShareError("channel_not_found");
    }
    await this.opts.storage.deleteShared(channel, id);
    const rec: SharedRecord = {
      id,
      channel,
      topic,
      groupId: ch.groupId,
      did: this.opts.selfDID,
      timestamp: this.now(),
      body: null,
      deleted: true,
    };
    await this.broadcast(ch, rec);
  }

  /** All non-expired shared records in a channel. */
  async list(channel: string): Promise<SharedRecord[]> {
    const recs = await this.opts.storage.listByChannel(channel);
    const now = this.now();
    return recs.filter((r) => !this.isExpired(r, now));
  }

  /**
   * All non-expired shared records for a group across all channels.
   * The infra layer does not check permissions — the app layer does.
   */
  async dump(groupId: string): Promise<SharedRecord[]> {
    const recs = await this.opts.storage.listByGroup(groupId);
    const now = this.now();
    return recs.filter((r) => !this.isExpired(r, now));
  }

  /**
   * Apply shared records using last-write-wins by timestamp. Returns the
   * number of records actually applied (newer than existing).
   */
  async restore(records: SharedRecord[]): Promise<number> {
    let applied = 0;
    for (const rec of records) {
      const existing = await this.opts.storage.getTimestamp(
        rec.channel,
        rec.id,
      );
      if (rec.timestamp <= existing) {
        continue;
      }
      await this.opts.storage.putShared(rec);
      applied++;
    }
    return applied;
  }

  /**
   * Physically remove expired records from a channel's storage. Returns the
   * count. Channels without retention (permanent) always return 0.
   */
  async purge(channel: string): Promise<number> {
    const ch = this.channels.get(channel);
    if (ch == null) {
      throw new GroupShareError("channel_not_found");
    }
    if ((ch.retentionMs ?? 0) === 0) {
      return 0;
    }
    const before = this.now() - (ch.retentionMs ?? 0);
    return await this.opts.storage.deleteExpired(channel, before);
  }

  /**
   * Process a shared record received from a peer: validate access policy,
   * role permission, and schema, then apply with last-write-wins.
   * Unknown channels and already-expired records are dropped silently.
   */
  async handleIncoming(payload: Uint8Array): Promise<void> {
    const rec = unmarshalSharedRecord(payload);
    const ch = this.channels.get(rec.channel);
    if (ch == null) {
      return; // unknown channel — skip silently
    }
    if (ch.access != null && !ch.access.canRead(rec.did)) {
      throw new GroupShareError("access_denied");
    }
    await this.checkPerm(ch, rec.did, "read");

    if (!rec.deleted && ch.schema != null) {
      try {
        ch.schema.validate(rec.body);
      } catch {
        throw new GroupShareError("schema_validation");
      }
    }

    if (this.isExpired(rec, this.now())) {
      return; // silently drop expired incoming records
    }

    const existing = await this.opts.storage.getTimestamp(rec.channel, rec.id);
    if (rec.timestamp <= existing) {
      return; // skip older
    }
    if (rec.deleted) {
      await this.opts.storage.deleteShared(rec.channel, rec.id);
      return;
    }
    await this.opts.storage.putShared(rec);
  }

  private async broadcastSubAnnouncement(
    ch: Channel,
    channel: string,
    topics: string[],
  ): Promise<void> {
    if (this.opts.memberResolver == null || this.opts.sendSubAnnounce == null) {
      return;
    }
    const members = await this.opts.memberResolver.memberDIDsForGroup(
      ch.groupId,
    );
    if (members.length === 0) {
      return;
    }
    const payload = marshalSubAnnouncement({
      did: this.opts.selfDID,
      channel,
      topics,
    });
    await this.opts.sendSubAnnounce(members, payload);
  }

  /**
   * Send a shared record to group members, filtering by subscription.
   * Unknown subscription (null) → include (safe default); a set
   * subscription that doesn't match the topic → skip.
   */
  private async broadcast(ch: Channel, rec: SharedRecord): Promise<void> {
    if (this.opts.memberResolver == null || this.opts.sendGroup == null) {
      return;
    }
    let members = await this.opts.memberResolver.memberDIDsForGroup(ch.groupId);
    if (members.length === 0) {
      return;
    }
    if (this.opts.remoteSubs != null) {
      const filtered: string[] = [];
      for (const did of members) {
        let topics: string[] | null;
        try {
          topics = await this.opts.remoteSubs.getSubscription(did, rec.channel);
        } catch {
          topics = null;
        }
        if (topics == null || topicMatches(topics, rec.topic)) {
          filtered.push(did);
        }
      }
      members = filtered;
    }
    if (members.length === 0) {
      return;
    }
    await this.opts.sendGroup(members, marshalSharedRecord(rec));
  }

  /**
   * Verify role-based permission for the DID on the channel. No-op when no
   * role DAG / perms / resolver are configured (allow all).
   */
  private async checkPerm(
    ch: Channel,
    did: string,
    op: "read" | "write" | "delete",
  ): Promise<void> {
    if (
      this.opts.roleDAG == null ||
      ch.perms == null ||
      this.opts.memberRoleResolver == null
    ) {
      return;
    }
    const memberRole = await this.opts.memberRoleResolver.memberRole(
      ch.groupId,
      did,
    );
    const required = ch.perms[op];
    if (required === "") {
      return;
    }
    if (!permissionCheck(this.opts.roleDAG, memberRole, required)) {
      throw new GroupShareError("access_denied");
    }
  }
}

/* ------------------------------------------------------------------ */
/* In-memory implementations                                            */
/* ------------------------------------------------------------------ */

/** In-memory SharedStorage for tests and validation. */
export class MemSharedStorage implements SharedStorage {
  private readonly records = new Map<string, SharedRecord>();

  private key(channel: string, id: string): string {
    return `${channel}\u0000${id}`;
  }

  async putShared(record: SharedRecord): Promise<void> {
    this.records.set(this.key(record.channel, record.id), copyShared(record));
  }

  async getShared(channel: string, id: string): Promise<SharedRecord | null> {
    const r = this.records.get(this.key(channel, id));
    return r == null ? null : copyShared(r);
  }

  async getTimestamp(channel: string, id: string): Promise<number> {
    return this.records.get(this.key(channel, id))?.timestamp ?? 0;
  }

  async deleteShared(channel: string, id: string): Promise<void> {
    this.records.delete(this.key(channel, id));
  }

  async listByChannel(channel: string): Promise<SharedRecord[]> {
    return [...this.records.values()]
      .filter((r) => r.channel === channel)
      .map(copyShared);
  }

  async listByGroup(groupId: string): Promise<SharedRecord[]> {
    return [...this.records.values()]
      .filter((r) => r.groupId === groupId)
      .map(copyShared);
  }

  async listByChannelAndTopic(
    channel: string,
    topic: string,
  ): Promise<SharedRecord[]> {
    return [...this.records.values()]
      .filter((r) => r.channel === channel && r.topic === topic)
      .map(copyShared);
  }

  async deleteExpired(channel: string, before: number): Promise<number> {
    let deleted = 0;
    for (const [k, r] of this.records) {
      if (r.channel === channel && r.timestamp < before) {
        this.records.delete(k);
        deleted++;
      }
    }
    return deleted;
  }
}

/** In-memory SubscriptionStore. */
export class MemSubscriptionStore implements SubscriptionStore {
  private readonly subs = new Map<string, string[]>();

  private key(did: string, channel: string): string {
    return `${did}\u0000${channel}`;
  }

  async setSubscription(
    did: string,
    channel: string,
    topics: string[],
  ): Promise<void> {
    this.subs.set(this.key(did, channel), [...topics]);
  }

  async getSubscription(
    did: string,
    channel: string,
  ): Promise<string[] | null> {
    return this.subs.get(this.key(did, channel)) ?? null;
  }

  async getAllSubscriptions(did: string): Promise<Map<string, string[]>> {
    const out = new Map<string, string[]>();
    for (const [k, v] of this.subs) {
      const [d, channel] = k.split("\u0000");
      if (d === did && channel !== undefined) {
        out.set(channel, v);
      }
    }
    return out;
  }
}

function copyShared(r: SharedRecord): SharedRecord {
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
