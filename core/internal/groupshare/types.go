// Package groupshare provides API-like data sharing between different DIDs
// in a group. Apps define Channels (named data streams) with schemas and
// access policies. Only data written through a Channel is shared with group
// members, following app-defined permissions.
package groupshare

import (
	"context"
	"time"
)

// SchemaValidator validates a record body before accepting it.
// Provided by the app.
type SchemaValidator interface {
	Validate(body []byte) error
}

// AccessPolicy controls who can read/write a channel.
// Provided by the app.
type AccessPolicy interface {
	CanWrite(did string) bool
	CanRead(did string) bool
}

// Channel defines a named data stream shared within a group.
type Channel struct {
	Name      string
	GroupID   string
	Schema    SchemaValidator // nil = accept any body
	Access    AccessPolicy    // nil = allow all
	Retention time.Duration   // 0 = permanent (master data)
}

// SharedRecord is the unit of data exchanged between group members.
type SharedRecord struct {
	ID        string `json:"id"`
	Channel   string `json:"channel"`
	Topic     string `json:"topic,omitempty"` // topic for subscription filtering
	GroupID   string `json:"group_id"`
	DID       string `json:"did"`       // writer's DID
	Timestamp int64  `json:"timestamp"` // milliseconds
	Body      []byte `json:"body"`
	Deleted   bool   `json:"deleted"`
}

// SharedStorage is the persistence interface for shared records.
type SharedStorage interface {
	PutShared(ctx context.Context, record *SharedRecord) error
	GetShared(ctx context.Context, channel, id string) (*SharedRecord, error)
	GetTimestamp(ctx context.Context, channel, id string) (int64, error)
	DeleteShared(ctx context.Context, channel, id string) error
	ListByChannel(ctx context.Context, channel string) ([]*SharedRecord, error)
	ListByGroup(ctx context.Context, groupID string) ([]*SharedRecord, error)
}

// MemberResolver returns the member DIDs for a group, excluding self.
type MemberResolver interface {
	MemberDIDsForGroup(ctx context.Context, groupID string) ([]string, error)
}

// SendGroupFunc sends a payload to a list of DIDs.
type SendGroupFunc func(ctx context.Context, memberDIDs []string, payload []byte) error

// SubAnnouncement is the message sent to peers to announce subscription changes.
type SubAnnouncement struct {
	DID     string   `json:"did"`
	Channel string   `json:"channel"`
	Topics  []string `json:"topics"`
}
