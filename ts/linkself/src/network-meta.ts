/**
 * Network membership propagation: a network's member/role state is itself
 * shared data. When an admin mutates membership (e.g. accepts a join), it
 * broadcasts a snapshot to the members so every node — especially the other
 * admins — converges on the same local view (network membership is held locally
 * per node; there is no central registry, see network-concept.md §1-2).
 *
 * Transport-free: marshal/unmarshal + a last-write-wins tracker over a
 * NetworkStore. The client wires this to the `network_meta` envelope type.
 *
 * Design: docs/spec/network-invitation.md §4
 */
import type { Network, NetworkStore } from "./network.js";

/** A membership snapshot for one network, versioned for LWW convergence. */
export interface NetworkMeta {
  networkId: string;
  suiteId: string;
  members: string[];
  memberRoles: Record<string, string>;
  /** Monotonic version (publisher's epoch ms). Higher wins on convergence. */
  epoch: number;
}

/** Build a snapshot for the given network at `epoch`. */
export function metaOf(net: Network, epoch: number): NetworkMeta {
  return {
    networkId: net.id,
    suiteId: net.suiteId,
    members: [...net.members],
    memberRoles: { ...net.memberRoles },
    epoch,
  };
}

export function marshalNetworkMeta(m: NetworkMeta): Uint8Array {
  return new TextEncoder().encode(
    JSON.stringify({
      networkId: m.networkId,
      suiteId: m.suiteId,
      members: m.members,
      memberRoles: m.memberRoles,
      epoch: m.epoch,
    }),
  );
}

export function unmarshalNetworkMeta(data: Uint8Array): NetworkMeta {
  const p = JSON.parse(new TextDecoder().decode(data)) as Partial<NetworkMeta>;
  if (
    typeof p.networkId !== "string" ||
    typeof p.suiteId !== "string" ||
    !Array.isArray(p.members) ||
    typeof p.memberRoles !== "object" ||
    p.memberRoles === null ||
    typeof p.epoch !== "number"
  ) {
    throw new Error("malformed network meta");
  }
  return {
    networkId: p.networkId,
    suiteId: p.suiteId,
    members: p.members as string[],
    memberRoles: p.memberRoles as Record<string, string>,
    epoch: p.epoch,
  };
}

/**
 * Applies membership snapshots to a NetworkStore with last-write-wins by epoch.
 * Authority (is the writer allowed to change membership?) is enforced by the
 * caller against local state — this layer only arbitrates versions.
 */
export class NetworkMetaTracker {
  private readonly epochs = new Map<string, number>();

  constructor(private readonly store: NetworkStore) {}

  /** The highest epoch applied for a network (0 if none). */
  lastEpoch(networkId: string): number {
    return this.epochs.get(networkId) ?? 0;
  }

  /**
   * Record an epoch reached by a local mutation we already applied to the store
   * ourselves (e.g. after publishing our own snapshot), so a concurrently
   * received older snapshot cannot regress it.
   */
  noteEpoch(networkId: string, epoch: number): void {
    if (epoch > this.lastEpoch(networkId)) {
      this.epochs.set(networkId, epoch);
    }
  }

  /**
   * Upsert the snapshot if it is newer than what we have. Returns true when
   * applied. Use for both the trusted bootstrap (join response) and ongoing
   * convergence (network_meta broadcasts).
   */
  async apply(meta: NetworkMeta): Promise<boolean> {
    if (meta.epoch <= this.lastEpoch(meta.networkId)) {
      return false;
    }
    await this.store.putNetwork({
      id: meta.networkId,
      suiteId: meta.suiteId,
      members: [...meta.members],
      memberRoles: { ...meta.memberRoles },
    });
    this.epochs.set(meta.networkId, meta.epoch);
    return true;
  }
}
