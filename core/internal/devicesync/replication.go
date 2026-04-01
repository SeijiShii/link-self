package devicesync

import (
	"context"
	"encoding/json"
	"time"
)

// RetentionPolicy configures ChangeLog pruning.
type RetentionPolicy struct {
	TimeBased bool          // true = prune by age, false = prune by count
	MaxAge    time.Duration // for time-based (default: 30 days)
	MaxCount  int           // for count-based (default: 10000)
}

// DefaultRetentionPolicy returns the default policy: time-based, 30 days.
func DefaultRetentionPolicy() *RetentionPolicy {
	return &RetentionPolicy{
		TimeBased: true,
		MaxAge:    30 * 24 * time.Hour,
	}
}

// ReplicationEngine provides transparent data replication between devices
// sharing the same DID. It wraps a DeviceStorage and broadcasts every
// local write to peer devices. Incoming changes are applied with
// last-write-wins conflict resolution.
type ReplicationEngine struct {
	Storage   DeviceStorage
	Send      SendFunc
	Peers     PeerProvider
	SelfDID   string
	Retention *RetentionPolicy
}

// Put stores a record locally and broadcasts the change to peer devices.
func (r *ReplicationEngine) Put(ctx context.Context, table, id string, body []byte) error {
	now := time.Now().UnixMilli()
	seq, err := r.Storage.Put(ctx, table, id, body, now)
	if err != nil {
		return err
	}

	entry := &ChangeEntry{
		Seq:       seq,
		Timestamp: now,
		Table:     table,
		RecordID:  id,
		Op:        OpPut,
		Body:      body,
	}

	r.enforceRetention(ctx, now)
	return r.broadcast(ctx, entry)
}

// Get retrieves a record from local storage.
func (r *ReplicationEngine) Get(ctx context.Context, table, id string) (*Record, error) {
	return r.Storage.Get(ctx, table, id)
}

// enforceRetention prunes old ChangeLog entries based on the retention policy.
func (r *ReplicationEngine) enforceRetention(ctx context.Context, nowMilli int64) {
	if r.Retention == nil {
		return
	}
	if r.Retention.TimeBased {
		maxAge := r.Retention.MaxAge
		if maxAge == 0 {
			maxAge = 30 * 24 * time.Hour
		}
		cutoff := nowMilli - maxAge.Milliseconds()
		// Find the first entry newer than cutoff and truncate before it.
		entries, err := r.Storage.ChangesSince(ctx, 0)
		if err != nil {
			return
		}
		var minSeq uint64
		for _, e := range entries {
			if e.Timestamp >= cutoff {
				minSeq = e.Seq
				break
			}
		}
		if minSeq > 0 {
			_ = r.Storage.TruncateChangeLog(ctx, minSeq)
		}
	} else {
		maxCount := r.Retention.MaxCount
		if maxCount == 0 {
			maxCount = 10000
		}
		latest, err := r.Storage.LatestSeq(ctx)
		if err != nil || latest == 0 {
			return
		}
		minSeq, err := r.Storage.MinSeq(ctx)
		if err != nil || minSeq == 0 {
			return
		}
		total := latest - minSeq + 1
		if total > uint64(maxCount) {
			cutSeq := latest - uint64(maxCount) + 1
			_ = r.Storage.TruncateChangeLog(ctx, cutSeq)
		}
	}
}

// Delete removes a record locally and broadcasts the change to peer devices.
func (r *ReplicationEngine) Delete(ctx context.Context, table, id string) error {
	now := time.Now().UnixMilli()
	seq, err := r.Storage.Delete(ctx, table, id, now)
	if err != nil {
		return err
	}

	entry := &ChangeEntry{
		Seq:       seq,
		Timestamp: now,
		Table:     table,
		RecordID:  id,
		Op:        OpDelete,
	}

	return r.broadcast(ctx, entry)
}

// List returns all records in a table from local storage.
func (r *ReplicationEngine) List(ctx context.Context, table string) ([]*Record, error) {
	return r.Storage.List(ctx, table)
}

// HandleIncoming processes a change entry received from a peer device.
// It applies last-write-wins: only records with a newer timestamp are applied.
func (r *ReplicationEngine) HandleIncoming(ctx context.Context, entry *ChangeEntry) error {
	existing, err := r.Storage.GetTimestamp(ctx, entry.Table, entry.RecordID)
	if err != nil {
		return err
	}

	if entry.Timestamp <= existing {
		return nil // skip older or equal
	}

	switch entry.Op {
	case OpPut:
		_, err = r.Storage.Put(ctx, entry.Table, entry.RecordID, entry.Body, entry.Timestamp)
	case OpDelete:
		_, err = r.Storage.Delete(ctx, entry.Table, entry.RecordID, entry.Timestamp)
	}
	return err
}

// SyncWith initiates a catch-up sync with a peer device.
// It exchanges the latest sequence number and sends/receives missing entries.
func (r *ReplicationEngine) SyncWith(ctx context.Context, peerID string) error {
	// TODO: implement full handshake protocol
	return nil
}

// broadcast sends a change entry to all peer devices.
func (r *ReplicationEngine) broadcast(ctx context.Context, entry *ChangeEntry) error {
	if r.Peers == nil || r.Send == nil {
		return nil
	}

	peers, err := r.Peers(ctx)
	if err != nil {
		return err
	}
	if len(peers) == 0 {
		return nil
	}

	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	for _, peer := range peers {
		if err := r.Send(ctx, peer, payload); err != nil {
			// Log but don't fail — store-and-forward will handle offline peers
			continue
		}
	}
	return nil
}
