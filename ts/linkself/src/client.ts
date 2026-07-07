/**
 * LinkSelf client assembly: wires the leaf node, device sync, group share,
 * and network layers together on top of a libp2p instance.
 * Browser-oriented port of pkg/linkself (Go). SQL-backed MyDB arrives with
 * the sqlite-wasm (OPFS) backend in M3; the KV device DB is available now.
 */
import { publicKeyToDID, type Identity } from "./did.js";
import {
  ReplicationEngine,
  MemDeviceStorage,
  unmarshalChangeEntry,
  type DeviceStorage,
} from "./devicesync.js";
import {
  TYPE_DEVICE_SYNC,
  TYPE_GROUP_SHARE,
  TYPE_SUB_ANNOUNCE,
  wrap,
} from "./envelope.js";
import {
  GroupShareLayer,
  MemSharedStorage,
  MemSubscriptionStore,
  type MemberRoleResolver,
  type SharedStorage,
  type SubscriptionStore,
} from "./groupshare.js";
import { LinkSelfNode, type Libp2pLike } from "./node.js";
import {
  MemNetworkStore,
  NetworkService,
  type NetworkStore,
} from "./network.js";
import { RoleDAG, type RoleDefs } from "./role.js";
import { didToPeerId } from "./did.js";

const SUBS_TABLE = "_groupshare_subs";

/**
 * SubscriptionStore backed by a devicesync ReplicationEngine: subscriptions
 * live in the "_groupshare_subs" table and replicate to same-DID devices.
 * Port of groupshare.DeviceSyncSubscriptionStore (Go), including the
 * "did::channel" record-ID encoding.
 */
export class DeviceSyncSubscriptionStore implements SubscriptionStore {
  constructor(private readonly engine: ReplicationEngine) {}

  private recordId(did: string, channel: string): string {
    return `${did}::${channel}`;
  }

  async setSubscription(
    did: string,
    channel: string,
    topics: string[],
  ): Promise<void> {
    const body = new TextEncoder().encode(JSON.stringify({ topics }));
    await this.engine.put(SUBS_TABLE, this.recordId(did, channel), body);
  }

  async getSubscription(
    did: string,
    channel: string,
  ): Promise<string[] | null> {
    const rec = await this.engine.get(SUBS_TABLE, this.recordId(did, channel));
    if (rec?.body == null) {
      return null;
    }
    const parsed = JSON.parse(new TextDecoder().decode(rec.body)) as {
      topics?: string[];
    };
    return parsed.topics ?? [];
  }

  async getAllSubscriptions(did: string): Promise<Map<string, string[]>> {
    const out = new Map<string, string[]>();
    const prefix = `${did}::`;
    for (const rec of await this.engine.list(SUBS_TABLE)) {
      if (!rec.id.startsWith(prefix) || rec.body == null) {
        continue;
      }
      try {
        const parsed = JSON.parse(new TextDecoder().decode(rec.body)) as {
          topics?: string[];
        };
        out.set(rec.id.slice(prefix.length), parsed.topics ?? []);
      } catch {
        // skip malformed records (parity with Go's continue-on-error)
      }
    }
    return out;
  }
}

export interface LinkSelfClientOptions {
  /** A started libp2p instance (transports/encryption configured by the app). */
  libp2p: Libp2pLike;
  identity: Identity;
  /** Role hierarchy; null = empty DAG (only "members" works). */
  roles?: RoleDefs | null;
  /** Role required for network management operations. Default "admin". */
  adminRole?: string;
  /** Storage backends; default to in-memory (OPFS-backed versions in M3). */
  deviceStorage?: DeviceStorage;
  sharedStorage?: SharedStorage;
  networkStore?: NetworkStore;
  remoteSubs?: SubscriptionStore;
  memberRoleResolver?: MemberRoleResolver | null;
  now?: () => number;
}

/**
 * Assembled LinkSelf client. Construct, then call start() after the libp2p
 * node is started. Envelope wiring matches the Go client:
 *   devicesync  → TYPE_DEVICE_SYNC to a specific peer device
 *   groupshare  → TYPE_GROUP_SHARE to group members
 *   subscriptions → TYPE_SUB_ANNOUNCE to group members
 *   auth success → announceAllSubscriptions (restores peer RemoteSubs)
 */
export class LinkSelfClient {
  readonly node: LinkSelfNode;
  readonly deviceSync: ReplicationEngine;
  readonly groupShare: GroupShareLayer;
  readonly network: NetworkService;
  readonly networkStore: NetworkStore;
  readonly remoteSubs: SubscriptionStore;
  readonly roleDAG: RoleDAG;
  readonly identity: Identity;

  constructor(opts: LinkSelfClientOptions) {
    this.identity = opts.identity;
    const selfDID = opts.identity.did;
    this.node = new LinkSelfNode(opts.libp2p, opts.identity);

    // DeviceSync: sends wrapped entries to specific peer devices.
    this.deviceSync = new ReplicationEngine({
      storage: opts.deviceStorage ?? new MemDeviceStorage(),
      selfDID,
      peers: async () => [], // single device until pairing registers peers
      send: async (peerId, payload) => {
        const { peerIdFromString } = await import("@libp2p/peer-id");
        await this.node.sendToPeerId(
          peerIdFromString(peerId),
          wrap(TYPE_DEVICE_SYNC, payload),
        );
      },
      now: opts.now,
    });

    // Network service with the role DAG.
    this.roleDAG = RoleDAG.build(opts.roles ?? {});
    this.networkStore = opts.networkStore ?? new MemNetworkStore();
    this.network = new NetworkService(
      this.networkStore,
      this.roleDAG,
      opts.adminRole ?? "admin",
    );

    // GroupShare: members resolved from the network store, excluding self.
    const memberResolver = {
      memberDIDsForGroup: async (groupId: string) => {
        const n = await this.networkStore.getNetwork(groupId);
        return (n?.members ?? []).filter((m) => m !== selfDID);
      },
    };
    this.remoteSubs = opts.remoteSubs ?? new MemSubscriptionStore();
    this.groupShare = new GroupShareLayer({
      storage: opts.sharedStorage ?? new MemSharedStorage(),
      memberResolver,
      selfDID,
      sendGroup: async (memberDIDs, payload) => {
        await this.sendToGroup(memberDIDs, wrap(TYPE_GROUP_SHARE, payload));
      },
      localSubs: new DeviceSyncSubscriptionStore(this.deviceSync),
      remoteSubs: this.remoteSubs,
      sendSubAnnounce: async (memberDIDs, payload) => {
        await this.sendToGroup(memberDIDs, wrap(TYPE_SUB_ANNOUNCE, payload));
      },
      roleDAG: this.roleDAG,
      memberRoleResolver:
        opts.memberRoleResolver === undefined
          ? {
              memberRole: async (groupId: string, memberDID: string) => {
                const n = await this.networkStore.getNetwork(groupId);
                return n?.memberRoles[memberDID] ?? "";
              },
            }
          : opts.memberRoleResolver,
      now: opts.now,
    });
  }

  /** Register protocol handlers and wire incoming envelope routing. */
  async start(): Promise<void> {
    await this.node.start();
    this.node.setOnDeviceSync((_peerDID, payload) => {
      void this.deviceSync
        .handleIncoming(unmarshalChangeEntry(payload))
        .catch((err) => {
          console.error("linkself: devicesync incoming:", err);
        });
    });
    this.node.setOnGroupShare((_peerDID, payload) => {
      void this.groupShare.handleIncoming(payload).catch((err) => {
        console.error("linkself: groupshare incoming:", err);
      });
    });
    this.node.setOnSubAnnounce((peerDID, payload) => {
      void this.groupShare
        .handleSubAnnouncement(peerDID, payload)
        .catch((err) => {
          console.error("linkself: sub announcement:", err);
        });
    });
    this.node.setOnAuthSuccess(() => {
      void this.groupShare.announceAllSubscriptions().catch((err) => {
        console.error("linkself: announce subscriptions:", err);
      });
    });
  }

  get did(): string {
    return this.identity.did;
  }

  /**
   * Send a raw payload to each member DID except self. Offline members are
   * queued via store-and-forward (parity with node.SendToGroup in Go).
   */
  async sendToGroup(memberDIDs: string[], payload: Uint8Array): Promise<void> {
    for (const did of memberDIDs) {
      if (did === this.identity.did) {
        continue;
      }
      try {
        await this.node.sendMessage(did, payload);
      } catch {
        // parity with Go: per-member send errors are ignored
      }
    }
  }

  /** Derive a peer's DID from its Ed25519 public key. */
  static didOfPublicKey = publicKeyToDID;

  /** Resolve the libp2p PeerId for a DID. */
  static peerIdOfDID = didToPeerId;
}
