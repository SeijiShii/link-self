package linkself

import (
	"context"

	"github.com/SeijiShii/link-self/core/internal/devicesync"
	"github.com/SeijiShii/link-self/core/internal/group"
	"github.com/SeijiShii/link-self/core/internal/groupshare"
)

// deviceDB wraps devicesync.ReplicationEngine to implement DeviceDB.
type deviceDB struct {
	engine *devicesync.ReplicationEngine
}

func (d *deviceDB) Put(ctx context.Context, table, recordID string, body []byte) error {
	return d.engine.Put(ctx, table, recordID, body)
}

func (d *deviceDB) Get(ctx context.Context, table, recordID string) (*Record, error) {
	rec, err := d.engine.Get(ctx, table, recordID)
	if err != nil || rec == nil {
		return nil, err
	}
	return &Record{
		ID:        rec.ID,
		Table:     rec.Table,
		Body:      rec.Body,
		Timestamp: rec.Timestamp,
	}, nil
}

func (d *deviceDB) Delete(ctx context.Context, table, recordID string) error {
	return d.engine.Delete(ctx, table, recordID)
}

func (d *deviceDB) List(ctx context.Context, table string) ([]*Record, error) {
	recs, err := d.engine.List(ctx, table)
	if err != nil {
		return nil, err
	}
	out := make([]*Record, len(recs))
	for i, r := range recs {
		out[i] = &Record{
			ID:        r.ID,
			Table:     r.Table,
			Body:      r.Body,
			Timestamp: r.Timestamp,
		}
	}
	return out, nil
}

// groupShareAPI wraps groupshare.GroupShareLayer to implement GroupShareAPI.
type groupShareAPI struct {
	layer *groupshare.GroupShareLayer
}

func (g *groupShareAPI) RegisterChannel(name, groupID string) error {
	return g.layer.RegisterChannel(&groupshare.Channel{
		Name:    name,
		GroupID: groupID,
	})
}

func (g *groupShareAPI) Put(ctx context.Context, channel, recordID string, body []byte) error {
	return g.layer.Put(ctx, channel, recordID, body)
}

func (g *groupShareAPI) Get(ctx context.Context, channel, recordID string) (*SharedRecord, error) {
	rec, err := g.layer.Get(ctx, channel, recordID)
	if err != nil || rec == nil {
		return nil, err
	}
	return &SharedRecord{
		ID:        rec.ID,
		Channel:   rec.Channel,
		GroupID:   rec.GroupID,
		DID:       rec.DID,
		Body:      rec.Body,
		Timestamp: rec.Timestamp,
	}, nil
}

func (g *groupShareAPI) Delete(ctx context.Context, channel, recordID string) error {
	return g.layer.Delete(ctx, channel, recordID)
}

func (g *groupShareAPI) List(ctx context.Context, channel string) ([]*SharedRecord, error) {
	recs, err := g.layer.List(ctx, channel)
	if err != nil {
		return nil, err
	}
	var out []*SharedRecord
	for _, r := range recs {
		if !r.Deleted {
			out = append(out, &SharedRecord{
				ID:        r.ID,
				Channel:   r.Channel,
				GroupID:   r.GroupID,
				DID:       r.DID,
				Body:      r.Body,
				Timestamp: r.Timestamp,
			})
		}
	}
	return out, nil
}

// groupAPI wraps group.Service + group.Store to implement GroupAPI.
type groupAPI struct {
	service *group.Service
	store   group.Store
	selfDID string
}

func (g *groupAPI) CreateGroup(ctx context.Context, memberDIDs []string) (string, error) {
	return g.service.CreateGroup(memberDIDs, []string{g.selfDID})
}

func (g *groupAPI) AddMember(ctx context.Context, groupID, memberDID string) error {
	return g.service.AddMember(groupID, memberDID)
}

func (g *groupAPI) Leave(ctx context.Context, groupID string) error {
	return g.service.Leave(groupID, g.selfDID)
}

func (g *groupAPI) ListGroups(ctx context.Context) ([]string, error) {
	return g.store.ListGroupIDsForMember(g.selfDID)
}

// memberResolverAdapter adapts group.Store to groupshare.MemberResolver.
type memberResolverAdapter struct {
	store   group.Store
	selfDID string
}

func (r *memberResolverAdapter) MemberDIDsForGroup(ctx context.Context, groupID string) ([]string, error) {
	g, err := r.store.GetGroup(groupID)
	if err != nil {
		return nil, err
	}
	var members []string
	for _, m := range g.Members {
		if m != r.selfDID {
			members = append(members, m)
		}
	}
	return members, nil
}
