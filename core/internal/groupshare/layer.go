package groupshare

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrChannelExists      = errors.New("channel already registered")
	ErrChannelNotFound    = errors.New("channel not registered")
	ErrAccessDenied       = errors.New("access denied")
	ErrSchemaValidation   = errors.New("schema validation failed")
)

// GroupShareLayer manages app-defined shared data channels and handles
// sending/receiving shared records between group members.
type GroupShareLayer struct {
	Storage        SharedStorage
	MemberResolver MemberResolver
	SendGroup      SendGroupFunc
	SelfDID        string
	channels       map[string]*Channel
}

// NewGroupShareLayer creates a new GroupShareLayer.
func NewGroupShareLayer(storage SharedStorage, resolver MemberResolver, send SendGroupFunc, selfDID string) *GroupShareLayer {
	return &GroupShareLayer{
		Storage:        storage,
		MemberResolver: resolver,
		SendGroup:      send,
		SelfDID:        selfDID,
		channels:       make(map[string]*Channel),
	}
}

// RegisterChannel registers a named data channel for sharing within a group.
func (l *GroupShareLayer) RegisterChannel(ch *Channel) error {
	if _, exists := l.channels[ch.Name]; exists {
		return ErrChannelExists
	}
	l.channels[ch.Name] = ch
	return nil
}

// Put writes a shared record to a channel and broadcasts to group members.
func (l *GroupShareLayer) Put(ctx context.Context, channel, id string, body []byte) error {
	ch, ok := l.channels[channel]
	if !ok {
		return ErrChannelNotFound
	}

	if ch.Access != nil && !ch.Access.CanWrite(l.SelfDID) {
		return ErrAccessDenied
	}

	now := time.Now().UnixMilli()
	rec := &SharedRecord{
		ID:        id,
		Channel:   channel,
		GroupID:   ch.GroupID,
		DID:       l.SelfDID,
		Timestamp: now,
		Body:      body,
	}

	if err := l.Storage.PutShared(ctx, rec); err != nil {
		return err
	}

	return l.broadcast(ctx, ch, rec)
}

// Get retrieves a shared record from a channel.
func (l *GroupShareLayer) Get(ctx context.Context, channel, id string) (*SharedRecord, error) {
	return l.Storage.GetShared(ctx, channel, id)
}

// Delete marks a shared record as deleted and broadcasts to group members.
func (l *GroupShareLayer) Delete(ctx context.Context, channel, id string) error {
	ch, ok := l.channels[channel]
	if !ok {
		return ErrChannelNotFound
	}

	if err := l.Storage.DeleteShared(ctx, channel, id); err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	rec := &SharedRecord{
		ID:        id,
		Channel:   channel,
		GroupID:   ch.GroupID,
		DID:       l.SelfDID,
		Timestamp: now,
		Deleted:   true,
	}

	return l.broadcast(ctx, ch, rec)
}

// List returns all shared records in a channel.
func (l *GroupShareLayer) List(ctx context.Context, channel string) ([]*SharedRecord, error) {
	return l.Storage.ListByChannel(ctx, channel)
}

// HandleIncoming processes a shared record received from a peer.
// Validates schema and access policy, then applies with last-write-wins.
func (l *GroupShareLayer) HandleIncoming(ctx context.Context, payload []byte) error {
	var rec SharedRecord
	if err := json.Unmarshal(payload, &rec); err != nil {
		return err
	}

	ch, ok := l.channels[rec.Channel]
	if !ok {
		// Unknown channel — skip silently
		return nil
	}

	// Check access: can this DID's data be accepted?
	if ch.Access != nil && !ch.Access.CanRead(rec.DID) {
		return ErrAccessDenied
	}

	// Validate schema (only for non-delete)
	if !rec.Deleted && ch.Schema != nil {
		if err := ch.Schema.Validate(rec.Body); err != nil {
			return ErrSchemaValidation
		}
	}

	// Last-write-wins
	existing, err := l.Storage.GetTimestamp(ctx, rec.Channel, rec.ID)
	if err != nil {
		return err
	}
	if rec.Timestamp <= existing {
		return nil // skip older
	}

	if rec.Deleted {
		return l.Storage.DeleteShared(ctx, rec.Channel, rec.ID)
	}
	return l.Storage.PutShared(ctx, &rec)
}

// broadcast sends a shared record to all group members.
func (l *GroupShareLayer) broadcast(ctx context.Context, ch *Channel, rec *SharedRecord) error {
	if l.MemberResolver == nil || l.SendGroup == nil {
		return nil
	}

	members, err := l.MemberResolver.MemberDIDsForGroup(ctx, ch.GroupID)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return nil
	}

	payload, err := json.Marshal(rec)
	if err != nil {
		return err
	}

	return l.SendGroup(ctx, members, payload)
}
