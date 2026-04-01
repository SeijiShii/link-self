// Package devicesync provides transparent data replication between devices
// sharing the same DID (identity). Apps use DeviceStorage like a local DB;
// writes are automatically broadcast to peer devices and applied with
// last-write-wins conflict resolution.
package devicesync

import "context"

// Op represents a storage operation type.
type Op int

const (
	OpPut    Op = iota // insert or update
	OpDelete           // delete
)

// ChangeEntry represents a single mutation in the change log.
// Every local write (Put or Delete) produces a ChangeEntry that is
// broadcast to peer devices for replication.
type ChangeEntry struct {
	Seq       uint64 `json:"seq"`       // monotonically increasing sequence per device
	Timestamp int64  `json:"timestamp"` // wall-clock milliseconds
	Table     string `json:"table"`     // logical table (namespace)
	RecordID  string `json:"record_id"` // unique record identifier within the table
	Op        Op     `json:"op"`        // Put or Delete
	Body      []byte `json:"body"`      // payload (only for OpPut)
}

// Record is a stored item returned by List/Get operations.
type Record struct {
	ID        string `json:"id"`
	Table     string `json:"table"`
	Body      []byte `json:"body"`
	Timestamp int64  `json:"timestamp"`
}

// DeviceStorage is the persistence interface for device-local data.
// The ReplicationEngine wraps this to add transparent sync.
// Implementations must be safe for concurrent use.
type DeviceStorage interface {
	// Put stores or updates a record. Returns the assigned sequence number.
	Put(ctx context.Context, table, id string, body []byte, timestamp int64) (uint64, error)

	// Get retrieves a single record. Returns nil, nil if not found.
	Get(ctx context.Context, table, id string) (*Record, error)

	// Delete removes a record. Returns the assigned sequence number.
	Delete(ctx context.Context, table, id string, timestamp int64) (uint64, error)

	// List returns all records in a table.
	List(ctx context.Context, table string) ([]*Record, error)

	// GetTimestamp returns the timestamp of an existing record.
	// Returns 0, nil if the record does not exist.
	GetTimestamp(ctx context.Context, table, id string) (int64, error)

	// AppendChange appends an entry to the change log.
	AppendChange(ctx context.Context, entry *ChangeEntry) error

	// ChangesSince returns all change entries with Seq > since, ordered by Seq.
	ChangesSince(ctx context.Context, since uint64) ([]*ChangeEntry, error)

	// LatestSeq returns the highest sequence number in the change log.
	// Returns 0 if no entries exist.
	LatestSeq(ctx context.Context) (uint64, error)

	// ListTables returns the names of all tables that contain at least one record.
	ListTables(ctx context.Context) ([]string, error)

	// MinSeq returns the lowest sequence number still present in the change log.
	// Returns 0 if the log is empty.
	MinSeq(ctx context.Context) (uint64, error)

	// TruncateChangeLog removes all change entries with Seq < minSeq.
	TruncateChangeLog(ctx context.Context, minSeq uint64) error
}

// SyncPeer abstracts the remote peer during a SyncWith handshake.
type SyncPeer struct {
	// LatestSeq returns the peer's highest synced sequence number.
	LatestSeq func(ctx context.Context) (uint64, error)
	// SendEntries delivers incremental change entries to the peer.
	SendEntries func(ctx context.Context, entries []*ChangeEntry) error
	// SendFullDump delivers a full data dump to the peer (fallback when gap detected).
	// May be nil; if nil and a gap is detected, SyncWith returns an error.
	SendFullDump func(ctx context.Context, records []*Record) error
}

// SendFunc sends a payload to a specific peer (identified by libp2p peer ID or address).
type SendFunc func(ctx context.Context, peerID string, payload []byte) error

// PeerProvider returns the list of peer device IDs (same DID, other devices).
type PeerProvider func(ctx context.Context) ([]string, error)
