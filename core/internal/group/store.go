// Package group implements the group concept: member set (2+), owner, leave, dissolve.
// See docs/group-concept.md. Storage is interface-based; domain rules live in this package.
package group

import "errors"

// Group holds group identity, members, and owners. Stored by Store implementations.
type Group struct {
	ID      string   // unique group id
	Members []string // member DIDs, at least 2 while group exists
	Owners  []string // owner DIDs, subset of Members
}

// Store is the interface for persisting group data. Callers depend only on this interface.
type Store interface {
	// ListGroupIDsForMember returns group IDs that include the given member DID.
	ListGroupIDsForMember(memberDID string) ([]string, error)
	// GetGroup returns the group by ID, or nil if not found.
	GetGroup(groupID string) (*Group, error)
	// CreateGroup saves a new group; implementation generates ID. Returns the new group ID.
	CreateGroup(members []string, ownerDIDs []string) (groupID string, err error)
	// UpdateGroup updates members and owners for the group.
	UpdateGroup(groupID string, members []string, ownerDIDs []string) error
	// DeleteGroup removes the group (e.g. when dissolved).
	DeleteGroup(groupID string) error
}

var (
	// ErrGroupNotFound is returned when a group ID does not exist.
	ErrGroupNotFound = errors.New("group not found")
)
