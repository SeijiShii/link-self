import { describe, expect, it } from "vitest";
import { canDelete, canRead, canWrite, check } from "../src/permission.js";
import { RoleDAG } from "../src/role.js";

const dag = RoleDAG.build({
  admin: { includes: ["editor"] },
  editor: { includes: [] },
});

describe("permission (mirroring permission_test.go)", () => {
  it("null permissions allow everything", () => {
    expect(canRead(dag, "nobody", null)).toBe(true);
    expect(canWrite(dag, "nobody", null)).toBe(true);
    expect(canDelete(dag, "nobody", null)).toBe(true);
  });

  it("'self' and 'owner' are never satisfied here (caller must handle)", () => {
    expect(check(dag, "admin", "self")).toBe(false);
    expect(check(dag, "admin", "owner")).toBe(false);
  });

  it("role-based checks delegate to the DAG", () => {
    const p = { read: "members", write: "editor", delete: "admin" };
    expect(canRead(dag, "", p)).toBe(true); // members: anyone
    expect(canWrite(dag, "editor", p)).toBe(true);
    expect(canWrite(dag, "admin", p)).toBe(true); // includes editor
    expect(canWrite(dag, "viewer", p)).toBe(false);
    expect(canDelete(dag, "editor", p)).toBe(false);
    expect(canDelete(dag, "admin", p)).toBe(true);
  });

  it("empty-string requirement is not satisfied by a normal role", () => {
    const p = { read: "members", write: "", delete: "" };
    expect(canWrite(dag, "admin", p)).toBe(false);
  });
});
