// Package role provides a role hierarchy as a directed acyclic graph (DAG).
// Roles are defined by applications and registered at startup. The DAG
// determines whether a member's role satisfies a required permission level.
package role

import "fmt"

// RoleDef defines a single role and its included (inherited) roles.
type RoleDef struct {
	Includes []string
}

// RoleDefs maps role names to their definitions.
type RoleDefs map[string]RoleDef

// DAG is a precomputed role hierarchy. Use NewDAG to create one.
type DAG struct {
	defs      RoleDefs
	ancestors map[string]map[string]bool // role → set of all transitively included roles
}

// NewDAG builds a role DAG from definitions. Returns an error if:
//   - A role includes an undefined role
//   - The graph contains a cycle
func NewDAG(defs RoleDefs) (*DAG, error) {
	d := &DAG{
		defs:      defs,
		ancestors: make(map[string]map[string]bool, len(defs)),
	}

	// Validate all includes reference defined roles.
	for name, def := range defs {
		for _, inc := range def.Includes {
			if _, ok := defs[inc]; !ok {
				return nil, fmt.Errorf("role %q includes undefined role %q", name, inc)
			}
		}
	}

	// Compute transitive closure with cycle detection.
	// State: 0=unvisited, 1=in-progress, 2=done
	state := make(map[string]int, len(defs))
	for name := range defs {
		if err := d.resolve(name, state); err != nil {
			return nil, err
		}
	}

	return d, nil
}

func (d *DAG) resolve(name string, state map[string]int) error {
	switch state[name] {
	case 2:
		return nil // already resolved
	case 1:
		return fmt.Errorf("cyclic role dependency involving %q", name)
	}

	state[name] = 1 // in-progress
	ancestors := make(map[string]bool)

	for _, inc := range d.defs[name].Includes {
		if err := d.resolve(inc, state); err != nil {
			return err
		}
		ancestors[inc] = true
		for a := range d.ancestors[inc] {
			ancestors[a] = true
		}
	}

	d.ancestors[name] = ancestors
	state[name] = 2 // done
	return nil
}

// HasRole reports whether memberRole satisfies requiredRole.
//
// Special values for requiredRole:
//   - "members": always satisfied (any role or no role)
//   - "self", "owner": not checked here (handled by the caller)
//
// Returns true if memberRole equals requiredRole, or transitively includes it.
func (d *DAG) HasRole(memberRole, requiredRole string) bool {
	if requiredRole == "members" {
		return true
	}

	if memberRole == requiredRole {
		return true
	}

	anc, ok := d.ancestors[memberRole]
	if !ok {
		return false
	}
	return anc[requiredRole]
}
