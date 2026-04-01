package linkself

import (
	"context"

	"github.com/SeijiShii/link-self/core/internal/devicesync"
	"github.com/SeijiShii/link-self/core/internal/group"
	"github.com/SeijiShii/link-self/core/internal/groupshare"
)

// myDB wraps devicesync.ReplicationEngine to implement DeviceDB.
type myDB struct {
	engine *devicesync.ReplicationEngine
}

func (d *myDB) Put(ctx context.Context, table, recordID string, body []byte) error {
	return d.engine.Put(ctx, table, recordID, body)
}

func (d *myDB) Get(ctx context.Context, table, recordID string) (*Record, error) {
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

func (d *myDB) Delete(ctx context.Context, table, recordID string) error {
	return d.engine.Delete(ctx, table, recordID)
}

func (d *myDB) List(ctx context.Context, table string) ([]*Record, error) {
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

func (d *myDB) Dump(ctx context.Context) ([]*Record, error) {
	tables, err := d.engine.Storage.ListTables(ctx)
	if err != nil {
		return nil, err
	}
	var out []*Record
	for _, table := range tables {
		recs, err := d.engine.List(ctx, table)
		if err != nil {
			return nil, err
		}
		for _, r := range recs {
			out = append(out, &Record{
				ID:        r.ID,
				Table:     r.Table,
				Body:      r.Body,
				Timestamp: r.Timestamp,
			})
		}
	}
	return out, nil
}

func (d *myDB) Restore(ctx context.Context, records []*Record) (int, error) {
	applied := 0
	for _, r := range records {
		existing, err := d.engine.Storage.GetTimestamp(ctx, r.Table, r.ID)
		if err != nil {
			return applied, err
		}
		if r.Timestamp > existing {
			if _, err := d.engine.Storage.Put(ctx, r.Table, r.ID, r.Body, r.Timestamp); err != nil {
				return applied, err
			}
			applied++
		}
	}
	return applied, nil
}

// sharedDB wraps groupshare.GroupShareLayer to implement SharedDB.
type sharedDB struct {
	layer *groupshare.GroupShareLayer
}

func (g *sharedDB) RegisterChannel(name, groupID string, opts ...ChannelOption) error {
	var cfg channelConfig
	for _, o := range opts {
		o(&cfg)
	}
	return g.layer.RegisterChannel(&groupshare.Channel{
		Name:      name,
		GroupID:   groupID,
		Retention: cfg.retention,
	})
}

func (g *sharedDB) Subscribe(channel string, topics []string) error {
	return g.layer.Subscribe(channel, topics)
}

func (g *sharedDB) Put(ctx context.Context, channel, topic, recordID string, body []byte) error {
	return g.layer.Put(ctx, channel, topic, recordID, body)
}

func (g *sharedDB) Get(ctx context.Context, channel, recordID string) (*SharedRecord, error) {
	rec, err := g.layer.Get(ctx, channel, recordID)
	if err != nil || rec == nil {
		return nil, err
	}
	return &SharedRecord{
		ID:        rec.ID,
		Channel:   rec.Channel,
		Topic:     rec.Topic,
		GroupID:   rec.GroupID,
		DID:       rec.DID,
		Body:      rec.Body,
		Timestamp: rec.Timestamp,
	}, nil
}

func (g *sharedDB) Delete(ctx context.Context, channel, topic, recordID string) error {
	return g.layer.Delete(ctx, channel, topic, recordID)
}

func (g *sharedDB) List(ctx context.Context, channel string) ([]*SharedRecord, error) {
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
				Topic:     r.Topic,
				GroupID:   r.GroupID,
				DID:       r.DID,
				Body:      r.Body,
				Timestamp: r.Timestamp,
			})
		}
	}
	return out, nil
}

func (g *sharedDB) Dump(ctx context.Context, groupID string) ([]*SharedRecord, error) {
	recs, err := g.layer.Dump(ctx, groupID)
	if err != nil {
		return nil, err
	}
	var out []*SharedRecord
	for _, r := range recs {
		out = append(out, &SharedRecord{
			ID:        r.ID,
			Channel:   r.Channel,
			Topic:     r.Topic,
			GroupID:   r.GroupID,
			DID:       r.DID,
			Body:      r.Body,
			Timestamp: r.Timestamp,
		})
	}
	return out, nil
}

func (g *sharedDB) Purge(ctx context.Context, channel string) (int, error) {
	return g.layer.Purge(ctx, channel)
}

func (g *sharedDB) Restore(ctx context.Context, records []*SharedRecord) (int, error) {
	internal := make([]*groupshare.SharedRecord, len(records))
	for i, r := range records {
		internal[i] = &groupshare.SharedRecord{
			ID:        r.ID,
			Channel:   r.Channel,
			Topic:     r.Topic,
			GroupID:   r.GroupID,
			DID:       r.DID,
			Body:      r.Body,
			Timestamp: r.Timestamp,
		}
	}
	return g.layer.Restore(ctx, internal)
}

// networkAPI wraps group.Service + group.Store to implement GroupAPI.
type networkAPI struct {
	service *group.Service
	store   group.Store
	selfDID string
}

func (g *networkAPI) CreateGroup(ctx context.Context, memberDIDs []string) (string, error) {
	return g.service.CreateGroup(memberDIDs, []string{g.selfDID})
}

func (g *networkAPI) AddMember(ctx context.Context, groupID, memberDID string) error {
	return g.service.AddMember(groupID, memberDID)
}

func (g *networkAPI) Leave(ctx context.Context, groupID string) error {
	return g.service.Leave(groupID, g.selfDID)
}

func (g *networkAPI) ListGroups(ctx context.Context) ([]string, error) {
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
