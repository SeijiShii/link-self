/**
 * Network management: a group of members within a suite, with role-gated
 * admin operations. Port of core/internal/network (Go).
 */
import type { RoleDAG } from "./role.js";

/** A group of members within a suite. */
export interface Network {
  id: string;
  suiteId: string;
  /** DID list. */
  members: string[];
  /** DID → role name ("" = no role assigned). */
  memberRoles: Record<string, string>;
}

export type NetworkErrorCode =
  | "network_not_found"
  | "not_member"
  | "no_permission"
  | "target_not_member"
  | "already_member";

const NETWORK_ERROR_MESSAGES: Record<NetworkErrorCode, string> = {
  network_not_found: "network not found",
  not_member: "not a member of this network",
  no_permission: "insufficient role permission",
  target_not_member: "target is not a member of this network",
  already_member: "target is already a member",
};

export class NetworkError extends Error {
  constructor(readonly code: NetworkErrorCode) {
    super(NETWORK_ERROR_MESSAGES[code]);
    this.name = "NetworkError";
  }
}

/** Persistence interface for networks. */
export interface NetworkStore {
  createNetwork(n: Network): Promise<string>;
  getNetwork(id: string): Promise<Network | null>;
  updateNetwork(id: string, n: Network): Promise<void>;
  deleteNetwork(id: string): Promise<void>;
  listForMember(memberDID: string): Promise<string[]>;
}

/**
 * Domain logic for network management. Permission checks use the role DAG:
 * only members whose role satisfies adminRole can add/kick members or
 * change roles.
 */
export class NetworkService {
  constructor(
    private readonly store: NetworkStore,
    private readonly dag: RoleDAG,
    private readonly adminRole: string,
  ) {}

  /** Create a new network with a single creator member who gets adminRole. */
  async create(suiteId: string, creatorDID: string): Promise<string> {
    return await this.store.createNetwork({
      id: "",
      suiteId,
      members: [creatorDID],
      memberRoles: { [creatorDID]: this.adminRole },
    });
  }

  /** Add a member. The requester must have adminRole. */
  async addMember(
    networkId: string,
    requesterDID: string,
    memberDID: string,
    memberRole: string,
  ): Promise<void> {
    const n = await this.getAndCheck(networkId, requesterDID);
    if (n.members.includes(memberDID)) {
      throw new NetworkError("already_member");
    }
    n.members.push(memberDID);
    n.memberRoles[memberDID] = memberRole;
    await this.store.updateNetwork(networkId, n);
  }

  /** Remove a member. If the last member leaves, the network is deleted. */
  async leave(networkId: string, memberDID: string): Promise<void> {
    const n = await this.store.getNetwork(networkId);
    if (n == null) {
      throw new NetworkError("network_not_found");
    }
    if (!n.members.includes(memberDID)) {
      throw new NetworkError("not_member");
    }
    n.members = n.members.filter((m) => m !== memberDID);
    delete n.memberRoles[memberDID];
    if (n.members.length === 0) {
      await this.store.deleteNetwork(networkId);
      return;
    }
    await this.store.updateNetwork(networkId, n);
  }

  /** Remove a member. The requester must have adminRole. */
  async kick(networkId: string, requesterDID: string, targetDID: string): Promise<void> {
    const n = await this.getAndCheck(networkId, requesterDID);
    if (!n.members.includes(targetDID)) {
      throw new NetworkError("target_not_member");
    }
    n.members = n.members.filter((m) => m !== targetDID);
    delete n.memberRoles[targetDID];
    if (n.members.length === 0) {
      await this.store.deleteNetwork(networkId);
      return;
    }
    await this.store.updateNetwork(networkId, n);
  }

  /** Change a member's role. The requester must have adminRole. */
  async setMemberRole(
    networkId: string,
    requesterDID: string,
    targetDID: string,
    newRole: string,
  ): Promise<void> {
    const n = await this.getAndCheck(networkId, requesterDID);
    if (!n.members.includes(targetDID)) {
      throw new NetworkError("target_not_member");
    }
    n.memberRoles[targetDID] = newRole;
    await this.store.updateNetwork(networkId, n);
  }

  private async getAndCheck(networkId: string, requesterDID: string): Promise<Network> {
    const n = await this.store.getNetwork(networkId);
    if (n == null) {
      throw new NetworkError("network_not_found");
    }
    const requesterRole = n.memberRoles[requesterDID] ?? "";
    if (!this.dag.hasRole(requesterRole, this.adminRole)) {
      throw new NetworkError("no_permission");
    }
    return n;
  }
}

/** In-memory NetworkStore for tests and validation. */
export class MemNetworkStore implements NetworkStore {
  private readonly networks = new Map<string, Network>();

  async createNetwork(n: Network): Promise<string> {
    const id = crypto.randomUUID();
    this.networks.set(id, { ...copyNetwork(n), id });
    return id;
  }

  async getNetwork(id: string): Promise<Network | null> {
    const n = this.networks.get(id);
    return n == null ? null : copyNetwork(n);
  }

  async updateNetwork(id: string, n: Network): Promise<void> {
    if (!this.networks.has(id)) {
      throw new NetworkError("network_not_found");
    }
    this.networks.set(id, { ...copyNetwork(n), id });
  }

  async deleteNetwork(id: string): Promise<void> {
    this.networks.delete(id);
  }

  async listForMember(memberDID: string): Promise<string[]> {
    return [...this.networks.values()].filter((n) => n.members.includes(memberDID)).map((n) => n.id);
  }
}

function copyNetwork(n: Network): Network {
  return { id: n.id, suiteId: n.suiteId, members: [...n.members], memberRoles: { ...n.memberRoles } };
}
