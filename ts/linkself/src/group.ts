/**
 * Group concept: member set (2+), owners, leave, dissolve, auto-promote.
 * Port of core/internal/group (Go). Storage is interface-based; domain
 * rules live here.
 */

/** Group identity, members, and owners. */
export interface Group {
  /** Unique group id. */
  id: string;
  /** Member DIDs, at least 2 while the group exists. */
  members: string[];
  /** Owner DIDs, subset of members. */
  owners: string[];
}

export type GroupErrorCode =
  | "group_not_found"
  | "too_few_members"
  | "not_member"
  | "not_owner"
  | "cannot_demote_other_owner"
  | "target_not_member"
  | "no_owner"
  | "already_member";

const GROUP_ERROR_MESSAGES: Record<GroupErrorCode, string> = {
  group_not_found: "group not found",
  too_few_members: "group requires at least 2 members",
  not_member: "not a member of the group",
  not_owner: "not an owner of the group",
  cannot_demote_other_owner: "owner cannot demote another owner",
  target_not_member: "target is not a member",
  no_owner: "cannot add member when group has no owners",
  already_member: "already a member of the group",
};

export class GroupError extends Error {
  constructor(readonly code: GroupErrorCode) {
    super(GROUP_ERROR_MESSAGES[code]);
    this.name = "GroupError";
  }
}

/** Interface for persisting group data. */
export interface GroupStore {
  /** Group IDs that include the given member DID. */
  listGroupIDsForMember(memberDID: string): Promise<string[]>;
  /** The group by ID, or null if not found. */
  getGroup(groupId: string): Promise<Group | null>;
  /** Save a new group; the implementation generates the ID. */
  createGroup(members: string[], ownerDIDs: string[]): Promise<string>;
  /** Update members and owners for the group. */
  updateGroup(groupId: string, members: string[], ownerDIDs: string[]): Promise<void>;
  /** Remove the group (e.g. when dissolved). */
  deleteGroup(groupId: string): Promise<void>;
}

/** Applies domain rules (member count, leave, owner permissions, auto-promote). */
export class GroupService {
  constructor(private readonly store: GroupStore) {}

  /** Create a group with at least 2 members. Owners can be 0, 1, or all. */
  async createGroup(members: string[], ownerDIDs: string[]): Promise<string> {
    if (members.length < 2) {
      throw new GroupError("too_few_members");
    }
    return await this.store.createGroup(members, ownerDIDs);
  }

  /** Remove the member. If the group would drop below 2 members, it is dissolved. */
  async leave(groupId: string, memberDID: string): Promise<void> {
    const g = await this.mustGet(groupId);
    if (!g.members.includes(memberDID)) {
      throw new GroupError("not_member");
    }
    const newMembers = g.members.filter((m) => m !== memberDID);
    let newOwners = g.owners.filter((o) => o !== memberDID);
    if (newMembers.length < 2) {
      await this.store.deleteGroup(groupId);
      return;
    }
    if (newOwners.length === 0) {
      newOwners = autoPromoteOneExcluding(newMembers, "");
    }
    await this.store.updateGroup(groupId, newMembers, newOwners);
  }

  /** Remove targetDID from the group. Caller must be an owner. */
  async kick(groupId: string, actorDID: string, targetDID: string): Promise<void> {
    const g = await this.mustGet(groupId);
    if (!g.owners.includes(actorDID)) {
      throw new GroupError("not_owner");
    }
    if (!g.members.includes(targetDID)) {
      throw new GroupError("target_not_member");
    }
    const newMembers = g.members.filter((m) => m !== targetDID);
    let newOwners = g.owners.filter((o) => o !== targetDID);
    if (newMembers.length < 2) {
      await this.store.deleteGroup(groupId);
      return;
    }
    if (newOwners.length === 0) {
      newOwners = autoPromoteOneExcluding(newMembers, "");
    }
    await this.store.updateGroup(groupId, newMembers, newOwners);
  }

  /** Add targetDID to owners. Caller must be an owner; target must be a member. */
  async appointOwner(groupId: string, actorDID: string, targetDID: string): Promise<void> {
    const g = await this.mustGet(groupId);
    if (!g.owners.includes(actorDID)) {
      throw new GroupError("not_owner");
    }
    if (!g.members.includes(targetDID)) {
      throw new GroupError("target_not_member");
    }
    if (g.owners.includes(targetDID)) {
      return;
    }
    await this.store.updateGroup(groupId, g.members, [...g.owners, targetDID]);
  }

  /**
   * Remove actorDID from owners. If that was the last owner, one remaining
   * member is auto-promoted.
   */
  async selfDemote(groupId: string, actorDID: string): Promise<void> {
    const g = await this.mustGet(groupId);
    if (!g.owners.includes(actorDID)) {
      throw new GroupError("not_owner");
    }
    let newOwners = g.owners.filter((o) => o !== actorDID);
    if (newOwners.length === 0) {
      newOwners = autoPromoteOneExcluding(g.members, actorDID);
    }
    await this.store.updateGroup(groupId, g.members, newOwners);
  }

  /**
   * Remove targetDID from owners. Caller must be an owner. Demoting another
   * owner is not allowed; demoting oneself delegates to selfDemote.
   */
  async demoteOwner(groupId: string, actorDID: string, targetDID: string): Promise<void> {
    const g = await this.mustGet(groupId);
    if (!g.owners.includes(actorDID)) {
      throw new GroupError("not_owner");
    }
    if (actorDID === targetDID) {
      await this.selfDemote(groupId, actorDID);
      return;
    }
    if (g.owners.includes(targetDID)) {
      throw new GroupError("cannot_demote_other_owner");
    }
  }

  /** Add newMemberDID to the group. Fails if the group has no owners. */
  async addMember(groupId: string, newMemberDID: string): Promise<void> {
    const g = await this.mustGet(groupId);
    if (g.owners.length === 0) {
      throw new GroupError("no_owner");
    }
    if (g.members.includes(newMemberDID)) {
      throw new GroupError("already_member");
    }
    await this.store.updateGroup(groupId, [...g.members, newMemberDID], g.owners);
  }

  private async mustGet(groupId: string): Promise<Group> {
    const g = await this.store.getGroup(groupId);
    if (g == null) {
      throw new GroupError("group_not_found");
    }
    return g;
  }
}

/** Pick one member (excluding excludeDID) so a group is never left with 0 owners. */
function autoPromoteOneExcluding(members: string[], excludeDID: string): string[] {
  for (const m of members) {
    if (m !== excludeDID) {
      return [m];
    }
  }
  return [];
}

/** In-memory GroupStore for tests and validation. */
export class MemGroupStore implements GroupStore {
  private readonly groups = new Map<string, Group>();

  async listGroupIDsForMember(memberDID: string): Promise<string[]> {
    const out: string[] = [];
    for (const g of this.groups.values()) {
      if (g.members.includes(memberDID)) {
        out.push(g.id);
      }
    }
    return out;
  }

  async getGroup(groupId: string): Promise<Group | null> {
    const g = this.groups.get(groupId);
    return g == null ? null : { id: g.id, members: [...g.members], owners: [...g.owners] };
  }

  async createGroup(members: string[], ownerDIDs: string[]): Promise<string> {
    const id = crypto.randomUUID();
    this.groups.set(id, { id, members: [...members], owners: [...ownerDIDs] });
    return id;
  }

  async updateGroup(groupId: string, members: string[], ownerDIDs: string[]): Promise<void> {
    const g = this.groups.get(groupId);
    if (g == null) {
      throw new GroupError("group_not_found");
    }
    this.groups.set(groupId, { id: groupId, members: [...members], owners: [...ownerDIDs] });
  }

  async deleteGroup(groupId: string): Promise<void> {
    this.groups.delete(groupId);
  }
}
