/**
 * Role hierarchy as a directed acyclic graph (DAG). Roles are defined by
 * applications and registered at startup; the DAG determines whether a
 * member's role satisfies a required permission level.
 * Port of core/internal/role (Go).
 */

/** Defines a single role and its included (inherited) roles. */
export interface RoleDef {
  includes: string[];
}

/** Maps role names to their definitions. */
export type RoleDefs = Record<string, RoleDef>;

export class RoleDefinitionError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "RoleDefinitionError";
  }
}

/** A precomputed role hierarchy. Use RoleDAG.build to create one. */
export class RoleDAG {
  private constructor(private readonly ancestors: Map<string, Set<string>>) {}

  /**
   * Build a role DAG from definitions. Throws RoleDefinitionError if a role
   * includes an undefined role or the graph contains a cycle.
   */
  static build(defs: RoleDefs): RoleDAG {
    for (const [name, def] of Object.entries(defs)) {
      for (const inc of def.includes) {
        if (!(inc in defs)) {
          throw new RoleDefinitionError(`role "${name}" includes undefined role "${inc}"`);
        }
      }
    }

    const ancestors = new Map<string, Set<string>>();
    // State: 0=unvisited, 1=in-progress, 2=done
    const state = new Map<string, number>();

    const resolve = (name: string): void => {
      if (state.get(name) === 2) return;
      if (state.get(name) === 1) {
        throw new RoleDefinitionError(`cyclic role dependency involving "${name}"`);
      }
      state.set(name, 1);
      const acc = new Set<string>();
      for (const inc of defs[name]!.includes) {
        resolve(inc);
        acc.add(inc);
        for (const a of ancestors.get(inc) ?? []) {
          acc.add(a);
        }
      }
      ancestors.set(name, acc);
      state.set(name, 2);
    };

    for (const name of Object.keys(defs)) {
      resolve(name);
    }
    return new RoleDAG(ancestors);
  }

  /**
   * Whether memberRole satisfies requiredRole.
   * Special values for requiredRole: "members" is always satisfied;
   * "self" / "owner" are not checked here (handled by the caller).
   */
  hasRole(memberRole: string, requiredRole: string): boolean {
    if (requiredRole === "members") return true;
    if (memberRole === requiredRole) return true;
    return this.ancestors.get(memberRole)?.has(requiredRole) ?? false;
  }
}
