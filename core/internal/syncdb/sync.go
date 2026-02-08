package syncdb

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SendGroupFunc sends a payload to the given member DIDs (e.g. node.SendToGroup). Called by SyncLayer.
type SendGroupFunc func(ctx context.Context, memberDIDs []string, payload []byte) error

// SyncLayer applies meta, persists, delivers to group, and applies incoming records with last-write-wins.
type SyncLayer struct {
	Storage   RecordStorage
	Resolver  MemberResolver
	SendGroup SendGroupFunc
	SelfDID   string
}

// NewSyncLayer returns a SyncLayer with the given dependencies.
func NewSyncLayer(storage RecordStorage, resolver MemberResolver, send SendGroupFunc, selfDID string) *SyncLayer {
	return &SyncLayer{
		Storage:   storage,
		Resolver:  resolver,
		SendGroup: send,
		SelfDID:   selfDID,
	}
}

// Put persists the record with meta (ID, DID, Timestamp), then sends to group members. Body and GroupID must be set by caller; ID may be empty to generate.
func (s *SyncLayer) Put(ctx context.Context, groupID string, body []byte) (*SyncRecord, error) {
	record := &SyncRecord{
		ID:        uuid.New().String(),
		GroupID:   groupID,
		DID:       s.SelfDID,
		Timestamp: time.Now().UnixMilli(),
		Body:      body,
	}
	if err := s.Storage.Put(ctx, record); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	memberDIDs, err := s.Resolver.MemberDIDsForGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	_ = s.SendGroup(ctx, memberDIDs, payload)
	return record, nil
}

// PutRecord persists and broadcasts an existing record (e.g. with pre-set ID). Used when applying incoming or re-sync.
func (s *SyncLayer) PutRecord(ctx context.Context, record *SyncRecord) error {
	return s.Storage.Put(ctx, record)
}

// HandleIncoming decodes the payload as SyncRecord and applies with last-write-wins: apply only if incoming Timestamp > existing.
func (s *SyncLayer) HandleIncoming(ctx context.Context, payload []byte) error {
	var record SyncRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return err
	}
	existing, err := s.Storage.GetTimestamp(ctx, record.ID)
	if err != nil {
		return err
	}
	if record.Timestamp <= existing {
		return nil
	}
	if record.Deleted {
		return s.Storage.Delete(ctx, record.ID)
	}
	return s.Storage.Put(ctx, &record)
}
