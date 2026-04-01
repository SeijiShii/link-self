package network

// Network represents a group of members within a suite.
type Network struct {
	ID          string
	SuiteID     string
	Members     []string          // DID list
	MemberRoles map[string]string // DID → role name (empty = no role assigned)
}

// Store is the persistence interface for networks.
type Store interface {
	CreateNetwork(n *Network) (string, error)
	GetNetwork(id string) (*Network, error)
	UpdateNetwork(id string, n *Network) error
	DeleteNetwork(id string) error
	ListForMember(memberDID string) ([]string, error)
}
