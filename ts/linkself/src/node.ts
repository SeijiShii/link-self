/**
 * LinkSelf leaf node: wires the auth and message protocols onto a libp2p
 * instance. Browser-oriented port of core/internal/node (Go) — a leaf that
 * dials out (WebSocket / Circuit Relay) and never listens.
 *
 * Not yet ported from Go: DHT-based peer discovery (browser leaves connect
 * via known addresses / relays instead), SendToGroup, device-sync specifics.
 */
import {
  respondToChallenge,
  verifyChallenge,
  mutualAuth,
  AUTH_PROTOCOL_ID,
  MUTUAL_AUTH_PROTOCOL_ID,
} from "./auth.js";
import { publicKeyToDID, didToPeerId, type Identity } from "./did.js";
import { encodeFrame, StreamReader } from "./framing.js";
import { MessageRouter, type MessageHandler } from "./router.js";
import { StoreForward } from "./storeforward.js";
import type { Connection, PeerId, Stream } from "@libp2p/interface";
import type { Multiaddr } from "@multiformats/multiaddr";

/** Message protocol: length-prefixed payload (matches the Go node). */
export const MESSAGE_PROTOCOL_ID = "/linkself/msg/1.0.0";

/** The subset of Libp2p that LinkSelfNode needs (kept narrow for testing). */
export interface Libp2pLike {
  peerId: PeerId;
  handle(
    protocol: string,
    handler: (stream: Stream, connection: Connection) => void | Promise<void>,
    options?: { runOnLimitedConnection?: boolean },
  ): Promise<void>;
  dial(
    addr: Multiaddr,
    options?: { signal?: AbortSignal },
  ): Promise<Connection>;
  getConnections(peerId?: PeerId): Connection[];
}

export class LinkSelfNode {
  readonly identity: Identity;
  readonly storeForward = new StoreForward();
  private readonly libp2p: Libp2pLike;
  private readonly router = new MessageRouter();
  private onAuthSuccess?: (peerDID: string) => void;
  /**
   * Remote peers whose DID we have authenticated, keyed by transport peer ID.
   * With mutual auth the transport key (peerId) no longer equals the DID key,
   * so the DID cannot be derived from the peer ID — it is recorded here on a
   * successful handshake and used for routing / device-peer lookup.
   */
  private readonly authedPeers = new Map<
    string,
    { did: string; peerId: PeerId }
  >();

  constructor(libp2p: Libp2pLike, identity: Identity) {
    this.libp2p = libp2p;
    this.identity = identity;
  }

  /** Register the LinkSelf protocol handlers. Call once after libp2p starts. */
  async start(): Promise<void> {
    await this.libp2p.handle(
      AUTH_PROTOCOL_ID,
      (stream, connection) => {
        void this.handleAuthStream(stream, connection);
      },
      { runOnLimitedConnection: true },
    );
    await this.libp2p.handle(
      MUTUAL_AUTH_PROTOCOL_ID,
      (stream, connection) => {
        void this.handleMutualAuthStream(stream, connection);
      },
      { runOnLimitedConnection: true },
    );
    await this.libp2p.handle(
      MESSAGE_PROTOCOL_ID,
      (stream, connection) => {
        void this.handleMessageStream(stream, connection);
      },
      { runOnLimitedConnection: true },
    );
  }

  setOnMessage(fn: MessageHandler): void {
    this.router.onMessage = fn;
  }

  setOnDeviceSync(fn: MessageHandler): void {
    this.router.onDeviceSync = fn;
  }

  setOnGroupShare(fn: MessageHandler): void {
    this.router.onGroupShare = fn;
  }

  setOnSubAnnounce(fn: MessageHandler): void {
    this.router.onSubAnnounce = fn;
  }

  /** Called after successful authentication (both incoming and outgoing). */
  setOnAuthSuccess(fn: (peerDID: string) => void): void {
    this.onAuthSuccess = fn;
  }

  /**
   * Dial the peer at the given multiaddr, authenticate against the expected
   * DID, then flush any store-and-forward queue for it. The addr must include
   * /p2p/<peer-id>.
   *
   * With `mutual: true` the peer is authenticated with MUTUAL_AUTH_PROTOCOL_ID
   * (peerId decoupled from DID) — used for peers that run their own transport
   * key, e.g. other devices of the same user. Otherwise the Go-compatible
   * one-way auth (peerId≡DID) is used. Port of node.ConnectToAddr (Go).
   */
  async connectToAddr(
    expectedDID: string,
    addr: Multiaddr,
    opts: { mutual?: boolean } = {},
  ): Promise<void> {
    const connection = await this.libp2p.dial(addr);
    if (opts.mutual) {
      const stream = await connection.newStream(MUTUAL_AUTH_PROTOCOL_ID, {
        runOnLimitedConnection: true,
      });
      const peerDID = await mutualAuth(
        stream,
        this.identity,
        true,
        expectedDID,
      );
      await stream.close();
      this.recordAuth(connection.remotePeer, peerDID);
      await this.afterAuth(peerDID, connection.remotePeer);
      return;
    }
    const stream = await connection.newStream(AUTH_PROTOCOL_ID, {
      runOnLimitedConnection: true,
    });
    await verifyChallenge(stream, expectedDID, connection.remotePeer);
    this.recordAuth(connection.remotePeer, expectedDID);
    await this.afterAuth(expectedDID, connection.remotePeer);
  }

  /**
   * Send a message to the peer(s) with the given DID over existing
   * connections. A DID may map to several connected devices (each with its
   * own transport key) — the message is delivered to all of them. If none are
   * connected, it is queued (store-and-forward) for the next successful auth.
   */
  async sendMessage(peerDID: string, payload: Uint8Array): Promise<void> {
    const peerIds = this.peerIdsForDID(peerDID);
    if (peerIds.length === 0) {
      // Fallback for one-way-auth / Go peers: the transport key is the DID key.
      const derived = didToPeerId(peerDID);
      if (this.libp2p.getConnections(derived).length > 0) {
        await this.sendToPeerId(derived, payload);
        return;
      }
      this.storeForward.queue(peerDID, payload);
      return;
    }
    for (const peerId of peerIds) {
      await this.sendToPeerId(peerId, payload);
    }
  }

  /** Send a raw payload directly to a connected peer by PeerId. */
  async sendToPeerId(peerId: PeerId, payload: Uint8Array): Promise<void> {
    const connection = this.libp2p.getConnections(peerId)[0];
    if (connection == null) {
      throw new Error(`no connection to ${peerId.toString()}`);
    }
    const stream = await connection.newStream(MESSAGE_PROTOCOL_ID, {
      runOnLimitedConnection: true,
    });
    stream.send(encodeFrame(payload));
    await stream.close();
  }

  private async afterAuth(peerDID: string, peerId: PeerId): Promise<void> {
    try {
      await this.storeForward.flushForDID(peerDID, async (payload) => {
        await this.sendToPeerId(peerId, payload);
      });
    } catch (err) {
      console.error(`linkself: flush store-and-forward for ${peerDID}:`, err);
    }
    this.onAuthSuccess?.(peerDID);
  }

  /** Responder side of incoming one-way auth, mirroring the Go stream handler. */
  private async handleAuthStream(
    stream: Stream,
    connection: Connection,
  ): Promise<void> {
    try {
      await respondToChallenge(stream, this.identity.privateKey);
      const peerDID = this.remoteDID(connection);
      if (peerDID != null) {
        this.recordAuth(connection.remotePeer, peerDID);
        await this.afterAuth(peerDID, connection.remotePeer);
      }
    } catch (err) {
      console.error("linkself: auth responder:", err);
    }
  }

  /** Responder side of incoming mutual auth (peerId decoupled from DID). */
  private async handleMutualAuthStream(
    stream: Stream,
    connection: Connection,
  ): Promise<void> {
    try {
      const peerDID = await mutualAuth(stream, this.identity, false);
      await stream.close();
      this.recordAuth(connection.remotePeer, peerDID);
      await this.afterAuth(peerDID, connection.remotePeer);
    } catch (err) {
      console.error("linkself: mutual auth responder:", err);
    }
  }

  private async handleMessageStream(
    stream: Stream,
    connection: Connection,
  ): Promise<void> {
    try {
      const payload = await new StreamReader(stream).readFrame();
      await stream.close();
      const peerDID = this.remoteDID(connection);
      if (peerDID != null) {
        this.router.dispatch(peerDID, payload);
      }
    } catch (err) {
      console.error("linkself: message stream:", err);
    }
  }

  /**
   * The DID authenticated for a connection. Prefers the DID verified via a
   * handshake (mutual auth, where peerId ≠ DID); falls back to deriving it
   * from the peer's Ed25519 transport key for one-way-auth / Go peers, where
   * the transport key IS the DID key.
   */
  private remoteDID(connection: Connection): string | undefined {
    const recorded = this.authedPeers.get(connection.remotePeer.toString());
    if (recorded != null) {
      return recorded.did;
    }
    const pub = connection.remotePeer.publicKey;
    if (pub == null || pub.type !== "Ed25519") {
      return undefined;
    }
    return publicKeyToDID(pub);
  }

  /** Record that a connection's remote peer authenticated as `did`. */
  private recordAuth(peerId: PeerId, did: string): void {
    this.authedPeers.set(peerId.toString(), { did, peerId });
  }

  /** Currently-connected peer IDs authenticated as `did`. */
  private peerIdsForDID(did: string): PeerId[] {
    const out: PeerId[] = [];
    for (const rec of this.authedPeers.values()) {
      if (
        rec.did === did &&
        this.libp2p.getConnections(rec.peerId).length > 0
      ) {
        out.push(rec.peerId);
      }
    }
    return out;
  }

  /**
   * Currently-connected peer IDs (as strings) authenticated as `did`. Used by
   * the client to feed devicesync the other devices of the same user.
   */
  peersForDID(did: string): string[] {
    return this.peerIdsForDID(did).map((p) => p.toString());
  }

  /** The DID authenticated for a connected peer, if a handshake recorded one. */
  didForPeer(peerId: PeerId): string | undefined {
    return this.authedPeers.get(peerId.toString())?.did;
  }
}
