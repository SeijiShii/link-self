package syncdb

import (
	"context"

	"github.com/SeijiShii/link-self/core/internal/group"
)

// MemberResolver returns member DIDs for a group (e.g. for SendToGroup). Sync layer uses this to get recipients.
type MemberResolver interface {
	MemberDIDsForGroup(ctx context.Context, groupID string) ([]string, error)
}

// GroupStoreResolver implements MemberResolver using group.Store, excluding selfDID from the list.
type GroupStoreResolver struct {
	Store   group.Store
	SelfDID string
}

// MemberDIDsForGroup returns the group's members excluding self, for delivery.
func (r *GroupStoreResolver) MemberDIDsForGroup(ctx context.Context, groupID string) ([]string, error) {
	g, err := r.Store.GetGroup(groupID)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, nil
	}
	var out []string
	for _, did := range g.Members {
		if did != r.SelfDID {
			out = append(out, did)
		}
	}
	return out, nil
}
