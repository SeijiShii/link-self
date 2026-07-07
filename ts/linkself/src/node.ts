/**
 * LinkSelf leaf node: wires the auth and message protocols onto a libp2p
 * instance. Browser-oriented port of core/internal/node (Go) — a leaf that
 * dials out (WebSocket / Circuit Relay) and never listens.
 *
 * Not yet ported from Go: DHT-based peer discovery (browser leaves connect
 * via known addresses / relays instead), SendToGroup, device-sync specifics.
 */
import { respondToChallenge, verifyChallenge, AUTH_PROTOCOL_ID } from "./auth.js";
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
  dial(addr: Multiaddr, options?: { signal?: AbortSignal }): Promise<Connection>;
  getConnections(peerId?: PeerId): Connection[];
}

export class LinkSelfNode {
  readonly identity: Identity;
  readonly storeForward = new StoreForward();
  private readonly libp2p: Libp2pLike;
  private readonly router = new MessageRouter();
  private onAuthSuccess?: (peerDID: string) => void;

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
   * Dial the peer at the given multiaddr, run initiator auth against the
   * expected DID, then flush any store-and-forward queue for it.
   * Port of node.ConnectToAddr (Go). The addr must include /p2p/<peer-id>.
   */
  async connectToAddr(expectedDID: string, addr: Multiaddr): Promise<void> {
    const connection = await this.libp2p.dial(addr);
    const stream = await connection.newStream(AUTH_PROTOCOL_ID, { runOnLimitedConnection: true });
    await verifyChallenge(stream, expectedDID, connection.remotePeer);
    await this.afterAuth(expectedDID, connection.remotePeer);
  }

  /**
   * Send a message to the peer with the given DID over an existing
   * connection. If the peer is not connected, the message is queued
   * (store-and-forward) and delivered after the next successful auth.
   */
  async sendMessage(peerDID: string, payload: Uint8Array): Promise<void> {
    const peerId = didToPeerId(peerDID);
    if (this.libp2p.getConnections(peerId).length === 0) {
      this.storeForward.queue(peerDID, payload);
      return;
    }
    await this.sendToPeerId(peerId, payload);
  }

  /** Send a raw payload directly to a connected peer by PeerId. */
  async sendToPeerId(peerId: PeerId, payload: Uint8Array): Promise<void> {
    const connection = this.libp2p.getConnections(peerId)[0];
    if (connection == null) {
      throw new Error(`no connection to ${peerId.toString()}`);
    }
    const stream = await connection.newStream(MESSAGE_PROTOCOL_ID, { runOnLimitedConnection: true });
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

  /** Responder side of incoming auth, mirroring the Go stream handler. */
  private async handleAuthStream(stream: Stream, connection: Connection): Promise<void> {
    try {
      await respondToChallenge(stream, this.identity.privateKey);
      const peerDID = this.remoteDID(connection);
      if (peerDID != null) {
        await this.afterAuth(peerDID, connection.remotePeer);
      }
    } catch (err) {
      console.error("linkself: auth responder:", err);
    }
  }

  private async handleMessageStream(stream: Stream, connection: Connection): Promise<void> {
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
   * Derive the remote peer's DID from its Ed25519 public key (embedded in
   * the peer ID / learned during the Noise handshake).
   */
  private remoteDID(connection: Connection): string | undefined {
    const pub = connection.remotePeer.publicKey;
    if (pub == null || pub.type !== "Ed25519") {
      return undefined;
    }
    return publicKeyToDID(pub);
  }
}
