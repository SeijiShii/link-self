/**
 * Store-and-Forward: queue messages by destination DID, flush when the peer
 * is reported online (e.g. after auth).
 * Port of core/internal/storeforward (Go).
 */

export type SendFunc = (payload: Uint8Array) => Promise<void>;

export class StoreForward {
  private readonly pending = new Map<string, Uint8Array[]>();

  /** Add a message for the given DID. Call when the peer is offline or unknown. */
  queue(did: string, payload: Uint8Array): void {
    const q = this.pending.get(did);
    if (q != null) {
      q.push(payload);
    } else {
      this.pending.set(did, [payload]);
    }
  }

  /**
   * Send all queued messages for the given DID using sendFn and clear the
   * queue. On the first send failure the remaining messages (including any
   * queued concurrently) are re-queued, and the error is rethrown.
   * Returns the number of messages sent.
   */
  async flushForDID(did: string, sendFn: SendFunc): Promise<number> {
    const msgs = this.pending.get(did) ?? [];
    this.pending.delete(did);
    let sent = 0;
    for (const payload of msgs) {
      try {
        await sendFn(payload);
      } catch (err) {
        const remaining = msgs.slice(sent).concat(this.pending.get(did) ?? []);
        this.pending.set(did, remaining);
        throw err;
      }
      sent++;
    }
    return sent;
  }

  /** Number of queued messages for the DID (or 0). */
  pendingCount(did: string): number {
    return this.pending.get(did)?.length ?? 0;
  }
}
