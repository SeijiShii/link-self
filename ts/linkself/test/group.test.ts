import { describe, expect, it } from "vitest";
import { GroupError, GroupService, MemGroupStore } from "../src/group.js";

async function expectGroupError(p: Promise<unknown>, code: string): Promise<void> {
  try {
    await p;
  } catch (err) {
    expect(err).toBeInstanceOf(GroupError);
    expect((err as GroupError).code).toBe(code);
    return;
  }
  throw new Error(`expected GroupError ${code}, but no error was thrown`);
}

function makeService() {
  const store = new MemGroupStore();
  return { store, service: new GroupService(store) };
}

describe("GroupService (mirroring group_test.go)", () => {
  it("rejects groups with fewer than 2 members", async () => {
    const { service } = makeService();
    await expectGroupError(service.createGroup(["did:a"], []), "too_few_members");
  });

  it("creates a group and lists it for members", async () => {
    const { store, service } = makeService();
    const id = await service.createGroup(["did:a", "did:b"], ["did:a"]);
    expect(await store.listGroupIDsForMember("did:a")).toEqual([id]);
    expect(await store.listGroupIDsForMember("did:c")).toEqual([]);
  });

  it("dissolves the group when leaving would drop below 2 members", async () => {
    const { store, service } = makeService();
    const id = await service.createGroup(["did:a", "did:b"], ["did:a"]);
    await service.leave(id, "did:b");
    expect(await store.getGroup(id)).toBeNull();
  });

  it("rejects leave by a non-member", async () => {
    const { service } = makeService();
    const id = await service.createGroup(["did:a", "did:b"], []);
    await expectGroupError(service.leave(id, "did:x"), "not_member");
  });

  it("auto-promotes one member when the last owner leaves", async () => {
    const { store, service } = makeService();
    const id = await service.createGroup(["did:a", "did:b", "did:c"], ["did:a"]);
    await service.leave(id, "did:a");
    const g = await store.getGroup(id);
    expect(g!.members).toEqual(["did:b", "did:c"]);
    expect(g!.owners).toHaveLength(1);
    expect(g!.members).toContain(g!.owners[0]);
  });

  it("kick requires the actor to be an owner", async () => {
    const { service } = makeService();
    const id = await service.createGroup(["did:a", "did:b", "did:c"], ["did:a"]);
    await expectGroupError(service.kick(id, "did:b", "did:c"), "not_owner");
  });

  it("kick removes the target; dissolves when dropping below 2", async () => {
    const { store, service } = makeService();
    const id = await service.createGroup(["did:a", "did:b", "did:c"], ["did:a"]);
    await service.kick(id, "did:a", "did:b");
    expect((await store.getGroup(id))!.members).toEqual(["did:a", "did:c"]);
    await service.kick(id, "did:a", "did:c");
    expect(await store.getGroup(id)).toBeNull();
  });

  it("kick rejects a non-member target", async () => {
    const { service } = makeService();
    const id = await service.createGroup(["did:a", "did:b"], ["did:a"]);
    await expectGroupError(service.kick(id, "did:a", "did:x"), "target_not_member");
  });

  it("appointOwner adds an owner (idempotent for existing owners)", async () => {
    const { store, service } = makeService();
    const id = await service.createGroup(["did:a", "did:b"], ["did:a"]);
    await service.appointOwner(id, "did:a", "did:b");
    expect((await store.getGroup(id))!.owners).toEqual(["did:a", "did:b"]);
    await service.appointOwner(id, "did:a", "did:b"); // no-op
    expect((await store.getGroup(id))!.owners).toEqual(["did:a", "did:b"]);
    await expectGroupError(service.appointOwner(id, "did:a", "did:x"), "target_not_member");
    await expectGroupError(service.appointOwner(id, "did:b2", "did:a"), "not_owner");
  });

  it("selfDemote auto-promotes another member when the last owner demotes", async () => {
    const { store, service } = makeService();
    const id = await service.createGroup(["did:a", "did:b"], ["did:a"]);
    await service.selfDemote(id, "did:a");
    const g = await store.getGroup(id);
    expect(g!.owners).toEqual(["did:b"]);
  });

  it("demoteOwner cannot demote another owner; self delegates to selfDemote", async () => {
    const { store, service } = makeService();
    const id = await service.createGroup(["did:a", "did:b"], ["did:a", "did:b"]);
    await expectGroupError(service.demoteOwner(id, "did:a", "did:b"), "cannot_demote_other_owner");
    await service.demoteOwner(id, "did:a", "did:a");
    expect((await store.getGroup(id))!.owners).toEqual(["did:b"]);
  });

  it("addMember requires an owner and rejects duplicates", async () => {
    const { store, service } = makeService();
    const noOwner = await service.createGroup(["did:a", "did:b"], []);
    await expectGroupError(service.addMember(noOwner, "did:c"), "no_owner");

    const id = await service.createGroup(["did:a", "did:b"], ["did:a"]);
    await service.addMember(id, "did:c");
    expect((await store.getGroup(id))!.members).toEqual(["did:a", "did:b", "did:c"]);
    await expectGroupError(service.addMember(id, "did:c"), "already_member");
  });

  it("operations on unknown groups fail with group_not_found", async () => {
    const { service } = makeService();
    await expectGroupError(service.leave("nope", "did:a"), "group_not_found");
    await expectGroupError(service.kick("nope", "did:a", "did:b"), "group_not_found");
    await expectGroupError(service.addMember("nope", "did:c"), "group_not_found");
  });
});
