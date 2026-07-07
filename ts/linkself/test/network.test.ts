import { describe, expect, it } from "vitest";
import { MemNetworkStore, NetworkError, NetworkService } from "../src/network.js";
import { RoleDAG } from "../src/role.js";

const dag = RoleDAG.build({
  admin: { includes: ["editor"] },
  editor: { includes: [] },
});

function makeService() {
  const store = new MemNetworkStore();
  return { store, service: new NetworkService(store, dag, "admin") };
}

async function expectNetworkError(p: Promise<unknown>, code: string): Promise<void> {
  try {
    await p;
  } catch (err) {
    expect(err).toBeInstanceOf(NetworkError);
    expect((err as NetworkError).code).toBe(code);
    return;
  }
  throw new Error(`expected NetworkError ${code}, but no error was thrown`);
}

describe("NetworkService (mirroring service_test.go)", () => {
  it("create makes a single-member network with the admin role", async () => {
    const { store, service } = makeService();
    const id = await service.create("jp.home-visit-suite", "did:creator");
    const n = await store.getNetwork(id);
    expect(n!.suiteId).toBe("jp.home-visit-suite");
    expect(n!.members).toEqual(["did:creator"]);
    expect(n!.memberRoles["did:creator"]).toBe("admin");
    expect(await store.listForMember("did:creator")).toEqual([id]);
  });

  it("addMember requires the admin role and rejects duplicates", async () => {
    const { store, service } = makeService();
    const id = await service.create("s", "did:admin");
    await service.addMember(id, "did:admin", "did:m1", "editor");
    expect((await store.getNetwork(id))!.memberRoles["did:m1"]).toBe("editor");

    await expectNetworkError(service.addMember(id, "did:m1", "did:m2", ""), "no_permission");
    await expectNetworkError(service.addMember(id, "did:admin", "did:m1", ""), "already_member");
  });

  it("a role that transitively includes admin can manage", async () => {
    const dag2 = RoleDAG.build({
      superadmin: { includes: ["admin"] },
      admin: { includes: [] },
    });
    const store = new MemNetworkStore();
    const service = new NetworkService(store, dag2, "admin");
    const id = await service.create("s", "did:root");
    await service.setMemberRole(id, "did:root", "did:root", "superadmin");
    await service.addMember(id, "did:root", "did:m1", ""); // superadmin ⊇ admin
    expect((await store.getNetwork(id))!.members).toContain("did:m1");
  });

  it("leave removes the member; the last member's leave deletes the network", async () => {
    const { store, service } = makeService();
    const id = await service.create("s", "did:admin");
    await service.addMember(id, "did:admin", "did:m1", "");
    await service.leave(id, "did:m1");
    expect((await store.getNetwork(id))!.members).toEqual(["did:admin"]);
    await service.leave(id, "did:admin");
    expect(await store.getNetwork(id)).toBeNull();

    await expectNetworkError(service.leave(id, "did:admin"), "network_not_found");
  });

  it("leave by a non-member fails", async () => {
    const { service } = makeService();
    const id = await service.create("s", "did:admin");
    await expectNetworkError(service.leave(id, "did:x"), "not_member");
  });

  it("kick requires admin and an existing target", async () => {
    const { store, service } = makeService();
    const id = await service.create("s", "did:admin");
    await service.addMember(id, "did:admin", "did:m1", "editor");
    await expectNetworkError(service.kick(id, "did:m1", "did:admin"), "no_permission");
    await expectNetworkError(service.kick(id, "did:admin", "did:x"), "target_not_member");
    await service.kick(id, "did:admin", "did:m1");
    expect((await store.getNetwork(id))!.members).toEqual(["did:admin"]);
  });

  it("setMemberRole updates the role for members only", async () => {
    const { store, service } = makeService();
    const id = await service.create("s", "did:admin");
    await service.addMember(id, "did:admin", "did:m1", "");
    await service.setMemberRole(id, "did:admin", "did:m1", "editor");
    expect((await store.getNetwork(id))!.memberRoles["did:m1"]).toBe("editor");
    await expectNetworkError(service.setMemberRole(id, "did:admin", "did:x", "editor"), "target_not_member");
    await expectNetworkError(service.setMemberRole(id, "did:m1", "did:m1", "admin"), "no_permission");
  });
});
