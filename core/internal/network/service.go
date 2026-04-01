package network

import "github.com/SeijiShii/link-self/core/internal/role"

// Service provides domain logic for network management.
// Permission checks use the role DAG: only members whose role
// satisfies adminRole can add/kick members or change roles.
type Service struct {
	store     Store
	dag       *role.DAG
	adminRole string // role required for management operations
}

// NewService creates a new network service.
// adminRole is the role name required for management operations (e.g. "admin").
func NewService(store Store, dag *role.DAG, adminRole string) *Service {
	return &Service{store: store, dag: dag, adminRole: adminRole}
}

// Create creates a new network with a single creator member who gets adminRole.
func (s *Service) Create(suiteID, creatorDID string) (string, error) {
	n := &Network{
		SuiteID:     suiteID,
		Members:     []string{creatorDID},
		MemberRoles: map[string]string{creatorDID: s.adminRole},
	}
	return s.store.CreateNetwork(n)
}

// AddMember adds a member to a network. requesterDID must have adminRole.
func (s *Service) AddMember(networkID, requesterDID, memberDID, memberRole string) error {
	n, err := s.getAndCheck(networkID, requesterDID)
	if err != nil {
		return err
	}

	for _, m := range n.Members {
		if m == memberDID {
			return ErrAlreadyMember
		}
	}

	n.Members = append(n.Members, memberDID)
	if n.MemberRoles == nil {
		n.MemberRoles = make(map[string]string)
	}
	n.MemberRoles[memberDID] = memberRole
	return s.store.UpdateNetwork(networkID, n)
}

// Leave removes a member from the network. If the last member leaves, the network is deleted.
func (s *Service) Leave(networkID, memberDID string) error {
	n, err := s.store.GetNetwork(networkID)
	if err != nil {
		return err
	}
	if n == nil {
		return ErrNetworkNotFound
	}

	idx := indexOf(n.Members, memberDID)
	if idx < 0 {
		return ErrNotMember
	}

	n.Members = append(n.Members[:idx], n.Members[idx+1:]...)
	delete(n.MemberRoles, memberDID)

	if len(n.Members) == 0 {
		return s.store.DeleteNetwork(networkID)
	}
	return s.store.UpdateNetwork(networkID, n)
}

// Kick removes a member. requesterDID must have adminRole.
func (s *Service) Kick(networkID, requesterDID, targetDID string) error {
	n, err := s.getAndCheck(networkID, requesterDID)
	if err != nil {
		return err
	}

	idx := indexOf(n.Members, targetDID)
	if idx < 0 {
		return ErrTargetNotMember
	}

	n.Members = append(n.Members[:idx], n.Members[idx+1:]...)
	delete(n.MemberRoles, targetDID)

	if len(n.Members) == 0 {
		return s.store.DeleteNetwork(networkID)
	}
	return s.store.UpdateNetwork(networkID, n)
}

// SetMemberRole changes a member's role. requesterDID must have adminRole.
func (s *Service) SetMemberRole(networkID, requesterDID, targetDID, newRole string) error {
	n, err := s.getAndCheck(networkID, requesterDID)
	if err != nil {
		return err
	}

	if indexOf(n.Members, targetDID) < 0 {
		return ErrTargetNotMember
	}

	n.MemberRoles[targetDID] = newRole
	return s.store.UpdateNetwork(networkID, n)
}

// getAndCheck loads a network and verifies requester has admin permission.
func (s *Service) getAndCheck(networkID, requesterDID string) (*Network, error) {
	n, err := s.store.GetNetwork(networkID)
	if err != nil {
		return nil, err
	}
	if n == nil {
		return nil, ErrNetworkNotFound
	}

	requesterRole := n.MemberRoles[requesterDID]
	if !s.dag.HasRole(requesterRole, s.adminRole) {
		return nil, ErrNoPermission
	}
	return n, nil
}

func indexOf(s []string, val string) int {
	for i, v := range s {
		if v == val {
			return i
		}
	}
	return -1
}
