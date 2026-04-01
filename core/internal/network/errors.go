package network

import "errors"

var (
	ErrNetworkNotFound = errors.New("network not found")
	ErrNotMember       = errors.New("not a member of this network")
	ErrNoPermission    = errors.New("insufficient role permission")
	ErrTooFewMembers   = errors.New("network requires at least 1 member")
	ErrTargetNotMember = errors.New("target is not a member of this network")
	ErrAlreadyMember   = errors.New("target is already a member")
)
