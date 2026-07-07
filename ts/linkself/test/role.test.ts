import { describe, expect, it } from "vitest";
import { RoleDAG, RoleDefinitionError } from "../src/role.js";

const HIERARCHY = {
  admin: { includes: ["editor"] },
  editor: { includes: ["member"] },
  member: { includes: [] },
  auditor: { includes: [] },
};

describe("RoleDAG (mirroring role_test.go)", () => {
  it("rejects includes of undefined roles", () => {
    expect(() => RoleDAG.build({ a: { includes: ["ghost"] } })).toThrow(RoleDefinitionError);
  });

  it("rejects cyclic dependencies", () => {
    expect(() =>
      RoleDAG.build({
        a: { includes: ["b"] },
        b: { includes: ["c"] },
        c: { includes: ["a"] },
      }),
    ).toThrow(RoleDefinitionError);
    expect(() => RoleDAG.build({ self: { includes: ["self"] } })).toThrow(RoleDefinitionError);
  });

  it("satisfies 'members' for any role, including unknown and empty", () => {
    const dag = RoleDAG.build(HIERARCHY);
    expect(dag.hasRole("admin", "members")).toBe(true);
    expect(dag.hasRole("stranger", "members")).toBe(true);
    expect(dag.hasRole("", "members")).toBe(true);
  });

  it("satisfies the same role", () => {
    const dag = RoleDAG.build(HIERARCHY);
    expect(dag.hasRole("editor", "editor")).toBe(true);
  });

  it("satisfies transitively included roles", () => {
    const dag = RoleDAG.build(HIERARCHY);
    expect(dag.hasRole("admin", "editor")).toBe(true);
    expect(dag.hasRole("admin", "member")).toBe(true); // via editor
    expect(dag.hasRole("editor", "member")).toBe(true);
  });

  it("does not satisfy unrelated or higher roles", () => {
    const dag = RoleDAG.build(HIERARCHY);
    expect(dag.hasRole("member", "editor")).toBe(false);
    expect(dag.hasRole("editor", "admin")).toBe(false);
    expect(dag.hasRole("admin", "auditor")).toBe(false);
    expect(dag.hasRole("stranger", "member")).toBe(false);
  });
});
