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
  TYPE_NETWORK_META,
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
import { multiaddr } from "@multiformats/multiaddr";
import { MyDB, wireSqlSync } from "./mydb.js";
import { LinkSelfNode, type Libp2pLike } from "./node.js";
import { SqlProxy, type SqlDatabase } from "./sqlproxy.js";
import {
  MemNetworkStore,
  NetworkService,
  type NetworkStore,
} from "./network.js";
import {
  marshalNetworkMeta,
  metaOf,
  NetworkMetaTracker,
  unmarshalNetworkMeta,
} from "./network-meta.js";
import { RoleDAG, type RoleDefs } from "./role.js";
import {
  decodeJoinRequest,
  decodeJoinResponse,
  encodeJoinRequest,
  encodeJoinResponse,
  JoinService,
  MemConsumedNonceStore,
  type ConsumedNonceStore,
  type JoinResponse,
} from "./join.js";
import type { Invite } from "./invitation.js";
import { didToPeerId } from "./did.js";
import {
  rosterDeviceDIDs,
  rosterHasDevice,
  type SignedRoster,
} from "./roster.js";

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
  /**
   * The transport/device identity — this device's key, which is also the
   * libp2p host key (so peerId ≡ this DID). In the two-layer model this is the
   * DEVICE identity; in single-device use it doubles as the account identity.
   */
  identity: Identity;
  /**
   * The account (user) identity, visible to the network and shared across the
   * user's devices. Signs the device roster. Defaults to `identity` (single
   * device = its own account). See roster.ts.
   */
  userIdentity?: Identity;
  /**
   * The user's signed device roster. When set, devicesync targets and accepts
   * only the sibling device DIDs it lists (roster membership = trust that a
   * device belongs to this user). Update with setRoster() as devices pair.
   */
  roster?: SignedRoster;
  /** Role hierarchy; null = empty DAG (only "members" works). */
  roles?: RoleDefs | null;
  /** Role required for network management operations. Default "admin". */
  adminRole?: string;
  /** Storage backends; default to in-memory (OPFS-backed versions in M3). */
  deviceStorage?: DeviceStorage;
  sharedStorage?: SharedStorage;
  networkStore?: NetworkStore;
  /** Tracks consumed invite nonces (single-use joins). Default in-memory. */
  consumedNonces?: ConsumedNonceStore;
  remoteSubs?: SubscriptionStore;
  memberRoleResolver?: MemberRoleResolver | null;
  /**
   * SQL backend for MyDB's SQL surface (e.g. SqliteWasmDatabase). Omit for
   * KV-only operation; detected SQL writes are mirrored into devicesync
   * (row-readback, matching the Go client).
   */
  sqlDatabase?: SqlDatabase | null;
  /**
   * FastStart hints: peers to dial (and authenticate) immediately on
   * start, skipping discovery — the browser counterpart of Go's
   * FastStart + KnownPeerHints (mobile-support §3.1.3). Best-effort:
   * unreachable entries are skipped. Persist a snapshotKnownPeers()
   * result across sessions and pass it back here.
   */
  knownPeers?: KnownPeer[];
  now?: () => number;
}

/** A previously seen peer: DID plus its last known multiaddrs. */
export interface KnownPeer {
  did: string;
  addrs: string[];
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
  /** Accepts incoming invitations (admin side of the join handshake). */
  readonly joinService: JoinService;
  /** Applies membership snapshots to the network store (LWW convergence). */
  readonly networkMeta: NetworkMetaTracker;
  readonly remoteSubs: SubscriptionStore;
  readonly roleDAG: RoleDAG;
  /** This device's transport identity (= libp2p host key, peerId ≡ this DID). */
  readonly identity: Identity;
  /** The account identity (user DID). Equals `identity` in single-device use. */
  readonly userIdentity: Identity;
  /** Unified data API (KV + SQL). SQL methods require options.sqlDatabase. */
  myDB!: MyDB;
  private readonly sqlDatabase: SqlDatabase | null;
  private readonly knownPeers: KnownPeer[];
  private readonly libp2p: Libp2pLike;
  /** The user's signed device roster (two-layer mode), or null. */
  private roster: SignedRoster | null;
  /** Clock for membership epochs (LWW). */
  private readonly nowFn: () => number;
  /** Role required to mutate/propagate membership. */
  private readonly adminRole: string;

  constructor(opts: LinkSelfClientOptions) {
    this.identity = opts.identity;
    this.userIdentity = opts.userIdentity ?? opts.identity;
    this.roster = opts.roster ?? null;
    this.sqlDatabase = opts.sqlDatabase ?? null;
    this.knownPeers = opts.knownPeers ?? [];
    this.nowFn = opts.now ?? Date.now;
    this.libp2p = opts.libp2p;
    // Devicesync/groupshare scope is the ACCOUNT (user) DID; transport is the
    // device DID. In single-device use the two coincide.
    const selfDID = this.userIdentity.did;
    this.node = new LinkSelfNode(opts.libp2p, opts.identity);

    // DeviceSync: sends wrapped entries to this user's other devices.
    this.deviceSync = new ReplicationEngine({
      storage: opts.deviceStorage ?? new MemDeviceStorage(),
      selfDID,
      // The user's other devices: roster device DIDs (peerId ≡ deviceDID) that
      // are currently connected, excluding this device. Falls back to the
      // authenticated-peer set when no roster is configured.
      peers: async () => this.deviceSyncPeers(),
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
    const adminRole = opts.adminRole ?? "admin";
    this.adminRole = adminRole;
    this.network = new NetworkService(
      this.networkStore,
      this.roleDAG,
      adminRole,
    );

    // Join handshake acceptance (admin side). Adds an invited DID to a network
    // after verifying the invite and admin authority; single-use via nonces.
    this.joinService = new JoinService(
      this.network,
      this.networkStore,
      this.roleDAG,
      adminRole,
      opts.consumedNonces ?? new MemConsumedNonceStore(),
      opts.now,
    );
    this.networkMeta = new NetworkMetaTracker(this.networkStore);

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
    // Assemble MyDB (KV always; SQL when a backend was provided).
    if (this.sqlDatabase != null) {
      const proxy = await SqlProxy.open(this.sqlDatabase);
      wireSqlSync(proxy, this.deviceSync);
      this.myDB = new MyDB(this.deviceSync, proxy);
    } else {
      this.myDB = new MyDB(this.deviceSync, null);
    }

    await this.node.start();
    this.node.setOnDeviceSync((peerDID, payload) => {
      // peerDID is the sender's DEVICE DID (peerId ≡ deviceDID). In two-layer
      // mode, only accept devicesync from a sibling device in the signed
      // roster — roster membership (user-key signed) is the trust anchor.
      if (this.roster != null && !rosterHasDevice(this.roster, peerDID)) {
        return;
      }
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
    this.node.setJoinAcceptor(async (peerDID, requestBytes) => {
      // Bind the membership to the transport-authenticated peer DID, not the
      // self-asserted inviteeDID in the request (prevents claiming another DID).
      const req = decodeJoinRequest(requestBytes);
      const res = await this.joinService.accept(this.userIdentity.did, {
        ...req,
        inviteeDID: peerDID,
      });
      if (res.ok) {
        // Converge the other members (esp. other admins) on the new roster.
        await this.publishMembership(res.network.networkId);
      }
      return encodeJoinResponse(res);
    });
    this.node.setOnNetworkMeta((peerDID, payload) => {
      void this.handleNetworkMeta(peerDID, payload).catch((err) => {
        console.error("linkself: network meta incoming:", err);
      });
    });

    await this.fastStart();
  }

  /**
   * FastStart: dial + authenticate every known peer in parallel,
   * best-effort (unreachable peers are skipped, first reachable addr per
   * peer wins). Connecting also flushes store-and-forward queues and
   * re-announces subscriptions via the auth-success hook.
   */
  private async fastStart(): Promise<void> {
    await Promise.all(
      this.knownPeers.map(async (peer) => {
        // Peers that share our DID are other devices of this user: they run
        // their own transport key, so authenticate with mutual auth (peerId
        // decoupled from DID) rather than the one-way peerId≡DID auth.
        const mutual = peer.did === this.identity.did;
        for (const addr of peer.addrs) {
          try {
            await this.node.connectToAddr(peer.did, multiaddr(addr), {
              mutual,
            });
            return;
          } catch {
            // try the next address
          }
        }
      }),
    );
  }

  /**
   * Snapshot the currently connected peers as FastStart hints. Persist
   * this (e.g. localStorage / OPFS) and pass it as options.knownPeers on
   * the next start — the browser counterpart of Go's peerstore
   * persistence (mobile-support §3.1.4).
   */
  snapshotKnownPeers(): KnownPeer[] {
    const byDID = new Map<string, Set<string>>();
    for (const conn of this.libp2p.getConnections()) {
      // Prefer the DID verified at auth (with device transport keys the peer
      // ID no longer encodes the DID); fall back to the transport key for
      // one-way-auth / Go peers where peerId ≡ DID.
      let did = this.node.didForPeer(conn.remotePeer);
      if (did == null) {
        const pub = conn.remotePeer.publicKey;
        if (pub == null || pub.type !== "Ed25519") {
          continue;
        }
        did = publicKeyToDID(pub);
      }
      const addrs = byDID.get(did) ?? new Set<string>();
      // remoteAddr already includes /p2p/<peer-id> or can be dialed as-is;
      // append the peer id when missing so connectToAddr can verify it.
      const addr = conn.remoteAddr.toString();
      addrs.add(
        addr.includes("/p2p/")
          ? addr
          : `${addr}/p2p/${conn.remotePeer.toString()}`,
      );
      byDID.set(did, addrs);
    }
    return [...byDID.entries()].map(([did, addrs]) => ({
      did,
      addrs: [...addrs],
    }));
  }

  /** The account (user) DID — what the network and group members see. */
  get did(): string {
    return this.userIdentity.did;
  }

  /** Update the signed device roster (e.g. after pairing a new device). */
  setRoster(roster: SignedRoster): void {
    this.roster = roster;
  }

  /**
   * The user's other devices to replicate to: roster device DIDs (peerId ≡
   * deviceDID) currently connected, excluding this device. Without a roster,
   * falls back to peers authenticated as the account DID.
   */
  private deviceSyncPeers(): string[] {
    if (this.roster == null) {
      return this.node.peersForDID(this.userIdentity.did);
    }
    const out: string[] = [];
    for (const deviceDID of rosterDeviceDIDs(this.roster)) {
      if (deviceDID === this.identity.did) continue; // skip self device
      const peerId = didToPeerId(deviceDID);
      if (this.libp2p.getConnections(peerId).length > 0) {
        out.push(peerId.toString());
      }
    }
    return out;
  }

  /**
   * Send a raw payload to each member DID except self. Offline members are
   * queued via store-and-forward (parity with node.SendToGroup in Go).
   */
  async sendToGroup(memberDIDs: string[], payload: Uint8Array): Promise<void> {
    for (const did of memberDIDs) {
      if (did === this.userIdentity.did) {
        continue;
      }
      try {
        await this.node.sendMessage(did, payload);
      } catch {
        // parity with Go: per-member send errors are ignored
      }
    }
  }

  /**
   * Invitee side of the join handshake: dial an admin at `addr` (a multiaddr
   * that includes /p2p/<admin-peer-id>, e.g. from `invite.relays`), present the
   * invite, and return the admin's decision. On success the response carries the
   * network snapshot; persisting it locally (so this device joins the data
   * plane) is the caller's responsibility (Slice 4).
   */
  async requestJoin(
    addr: string,
    invite: Invite,
    displayName: string,
  ): Promise<JoinResponse> {
    const requestBytes = encodeJoinRequest({
      v: 1,
      invite,
      inviteeDID: this.userIdentity.did,
      displayName,
    });
    const responseBytes = await this.node.requestJoin(
      multiaddr(addr),
      requestBytes,
    );
    const res = decodeJoinResponse(responseBytes);
    if (res.ok) {
      // Trusted bootstrap: the snapshot arrived over the authenticated join
      // stream, so seed the local network store from it (this device joins the
      // data plane). Subsequent network_meta broadcasts converge from here.
      await this.networkMeta.apply({
        ...res.network,
        epoch: this.nowFn(),
      });
    }
    return res;
  }

  /**
   * Broadcast the current membership of a network to its members so they
   * converge (LWW). Called after admin mutations (e.g. accepting a join); the
   * caller must be an admin for peers to accept the update.
   */
  async publishMembership(networkId: string): Promise<void> {
    const net = await this.networkStore.getNetwork(networkId);
    if (net == null) {
      return;
    }
    const meta = metaOf(net, this.nowFn());
    this.networkMeta.noteEpoch(networkId, meta.epoch);
    await this.sendToGroup(
      net.members,
      wrap(TYPE_NETWORK_META, marshalNetworkMeta(meta)),
    );
  }

  /**
   * Apply an incoming membership snapshot. Only accepted when the sender is an
   * admin of the network in our current local view (prevents a non-admin member
   * forging membership); the initial network is bootstrapped via the trusted
   * join response, not here.
   */
  private async handleNetworkMeta(
    peerDID: string,
    payload: Uint8Array,
  ): Promise<void> {
    const meta = unmarshalNetworkMeta(payload);
    const net = await this.networkStore.getNetwork(meta.networkId);
    if (net == null) {
      return; // unknown network — bootstrap happens via the join response
    }
    const writerRole = net.memberRoles[peerDID] ?? "";
    if (!this.roleDAG.hasRole(writerRole, this.adminRole)) {
      return; // sender is not an admin — ignore
    }
    await this.networkMeta.apply(meta);
  }

  /** Derive a peer's DID from its Ed25519 public key. */
  static didOfPublicKey = publicKeyToDID;

  /**
   * Resolve the libp2p PeerId a DID would have if the DID key were used as the
   * transport key. Valid only for one-way-auth / Go peers (peerId ≡ DID); with
   * per-device transport keys a DID maps to several distinct peer IDs, so use
   * the node's authenticated peer lookup instead.
   */
  static peerIdOfDID = didToPeerId;
}
