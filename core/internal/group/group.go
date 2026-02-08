package group

import "errors"

var (
	ErrTooFewMembers          = errors.New("group requires at least 2 members")
	ErrNotMember              = errors.New("not a member of the group")
	ErrNotOwner               = errors.New("not an owner of the group")
	ErrCannotDemoteOtherOwner = errors.New("owner cannot demote another owner")
	ErrTargetNotMember        = errors.New("target is not a member")
	ErrNoOwner                = errors.New("cannot add member when group has no owners")
	ErrAlreadyMember          = errors.New("already a member of the group")
)

// Service applies domain rules (member count, leave, owner permissions, auto-promote) using Store.
type Service struct {
	store Store
}

// NewService returns a domain service that uses the given store.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// CreateGroup creates a group with at least 2 members. Owners can be 0, 1, or all (2-person: 隠匿してよい).
func (s *Service) CreateGroup(members []string, ownerDIDs []string) (groupID string, err error) {
	if len(members) < 2 {
		return "", ErrTooFewMembers
	}
	return s.store.CreateGroup(members, ownerDIDs)
}

// Leave removes the member from the group. If the group becomes 1 member, it is dissolved (deleted).
func (s *Service) Leave(groupID string, memberDID string) error {
	g, err := s.store.GetGroup(groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return ErrGroupNotFound
	}
	if !contains(g.Members, memberDID) {
		return ErrNotMember
	}
	newMembers := without(g.Members, memberDID)
	newOwners := without(g.Owners, memberDID)
	if len(newMembers) < 2 {
		return s.store.DeleteGroup(groupID)
	}
	if len(newOwners) == 0 {
		newOwners = autoPromoteOneExcluding(newMembers, "")
	}
	return s.store.UpdateGroup(groupID, newMembers, newOwners)
}

// Kick removes targetDID from the group. Caller must be an owner.
func (s *Service) Kick(groupID string, actorDID string, targetDID string) error {
	g, err := s.store.GetGroup(groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return ErrGroupNotFound
	}
	if !contains(g.Owners, actorDID) {
		return ErrNotOwner
	}
	if !contains(g.Members, targetDID) {
		return ErrTargetNotMember
	}
	newMembers := without(g.Members, targetDID)
	newOwners := without(g.Owners, targetDID)
	if len(newMembers) < 2 {
		return s.store.DeleteGroup(groupID)
	}
	if len(newOwners) == 0 {
		newOwners = autoPromoteOneExcluding(newMembers, "")
	}
	return s.store.UpdateGroup(groupID, newMembers, newOwners)
}

// AppointOwner adds targetDID to owners. Caller must be an owner; target must be a member.
func (s *Service) AppointOwner(groupID string, actorDID string, targetDID string) error {
	g, err := s.store.GetGroup(groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return ErrGroupNotFound
	}
	if !contains(g.Owners, actorDID) {
		return ErrNotOwner
	}
	if !contains(g.Members, targetDID) {
		return ErrTargetNotMember
	}
	if contains(g.Owners, targetDID) {
		return nil
	}
	newOwners := append([]string{}, g.Owners...)
	newOwners = append(newOwners, targetDID)
	return s.store.UpdateGroup(groupID, g.Members, newOwners)
}

// SelfDemote removes actorDID from owners. If that was the last owner, one remaining member is auto-promoted.
func (s *Service) SelfDemote(groupID string, actorDID string) error {
	g, err := s.store.GetGroup(groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return ErrGroupNotFound
	}
	if !contains(g.Owners, actorDID) {
		return ErrNotOwner
	}
	newOwners := without(g.Owners, actorDID)
	if len(newOwners) == 0 {
		newOwners = autoPromoteOneExcluding(g.Members, actorDID)
	}
	return s.store.UpdateGroup(groupID, g.Members, newOwners)
}

// DemoteOwner removes targetDID from owners. Caller must be an owner. Fails if target is another owner (他オーナー降格不可).
func (s *Service) DemoteOwner(groupID string, actorDID string, targetDID string) error {
	g, err := s.store.GetGroup(groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return ErrGroupNotFound
	}
	if !contains(g.Owners, actorDID) {
		return ErrNotOwner
	}
	if actorDID == targetDID {
		return s.SelfDemote(groupID, actorDID)
	}
	if contains(g.Owners, targetDID) {
		return ErrCannotDemoteOtherOwner
	}
	return nil
}

// AddMember adds newMemberDID to the group. Fails if the group has no owners (オーナー0では3人目招待不可).
func (s *Service) AddMember(groupID string, newMemberDID string) error {
	g, err := s.store.GetGroup(groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return ErrGroupNotFound
	}
	if len(g.Owners) == 0 {
		return ErrNoOwner
	}
	if contains(g.Members, newMemberDID) {
		return ErrAlreadyMember
	}
	newMembers := append([]string{}, g.Members...)
	newMembers = append(newMembers, newMemberDID)
	return s.store.UpdateGroup(groupID, newMembers, g.Owners)
}

func contains(slice []string, x string) bool {
	for _, v := range slice {
		if v == x {
			return true
		}
	}
	return false
}

func without(slice []string, x string) []string {
	var out []string
	for _, v := range slice {
		if v != x {
			out = append(out, v)
		}
	}
	return out
}

// autoPromoteOneExcluding picks one member from the list, excluding excludeDID (e.g. the one who just left/demoted). Never leave 0 owners.
func autoPromoteOneExcluding(members []string, excludeDID string) []string {
	for _, m := range members {
		if m != excludeDID {
			return []string{m}
		}
	}
	return nil
}
