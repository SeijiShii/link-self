package groupshare

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/SeijiShii/link-self/core/internal/permission"
	"github.com/SeijiShii/link-self/core/internal/role"
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
	Storage            SharedStorage
	MemberResolver     MemberResolver
	SendGroup          SendGroupFunc
	SelfDID            string
	LocalSubs          SubscriptionStore  // this device's subscriptions; nil = no persistence
	RemoteSubs         SubscriptionStore  // remote peers' subscriptions; nil = send to all
	SendSubAnnounce    SendGroupFunc      // sends sub announcements; nil = no broadcast
	roleDAG            *role.DAG          // nil = skip role-based permission checks
	memberRoleResolver MemberRoleResolver // nil = skip role-based permission checks
	channels           map[string]*Channel
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

// Subscribe declares which topics this device wants for a channel.
// ["*"] means all topics, [] means unsubscribe.
// Stores locally and broadcasts a SubAnnouncement to group members.
func (l *GroupShareLayer) Subscribe(channel string, topics []string) error {
	ch, ok := l.channels[channel]
	if !ok {
		return ErrChannelNotFound
	}
	if l.LocalSubs != nil {
		if err := l.LocalSubs.SetSubscription(l.SelfDID, channel, topics); err != nil {
			return err
		}
	}
	return l.broadcastSubAnnouncement(ch, channel, topics)
}

// HandleSubAnnouncement processes a subscription announcement from a peer.
// Validates that senderDID matches the announcement DID to prevent spoofing.
func (l *GroupShareLayer) HandleSubAnnouncement(senderDID string, payload []byte) error {
	var ann SubAnnouncement
	if err := json.Unmarshal(payload, &ann); err != nil {
		return err
	}
	if ann.DID != senderDID {
		return errors.New("subscription announcement DID mismatch")
	}
	if l.RemoteSubs != nil {
		return l.RemoteSubs.SetSubscription(ann.DID, ann.Channel, ann.Topics)
	}
	return nil
}

// broadcastSubAnnouncement sends a SubAnnouncement to all group members.
func (l *GroupShareLayer) broadcastSubAnnouncement(ch *Channel, channel string, topics []string) error {
	if l.MemberResolver == nil || l.SendSubAnnounce == nil {
		return nil
	}
	members, err := l.MemberResolver.MemberDIDsForGroup(context.Background(), ch.GroupID)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return nil
	}
	ann := &SubAnnouncement{
		DID:     l.SelfDID,
		Channel: channel,
		Topics:  topics,
	}
	payload, err := json.Marshal(ann)
	if err != nil {
		return err
	}
	return l.SendSubAnnounce(context.Background(), members, payload)
}

// AnnounceAllSubscriptions re-broadcasts all local subscriptions to group members.
// Called after peer reconnection to restore RemoteSubs state on the peer side.
func (l *GroupShareLayer) AnnounceAllSubscriptions(ctx context.Context) error {
	if l.LocalSubs == nil {
		return nil
	}
	subs, err := l.LocalSubs.GetAllSubscriptions(l.SelfDID)
	if err != nil {
		return err
	}
	for channel, topics := range subs {
		ch, ok := l.channels[channel]
		if !ok {
			continue
		}
		if err := l.broadcastSubAnnouncement(ch, channel, topics); err != nil {
			return err
		}
	}
	return nil
}

// IsExpired returns true if the record has exceeded the channel's retention period.
// Returns false if the channel has no retention (permanent) or if the channel is unknown.
func (l *GroupShareLayer) IsExpired(rec *SharedRecord, now int64) bool {
	ch, ok := l.channels[rec.Channel]
	if !ok || ch.Retention == 0 {
		return false
	}
	return now >= rec.Timestamp+ch.Retention.Milliseconds()
}

// Put writes a shared record to a channel and broadcasts to group members.
func (l *GroupShareLayer) Put(ctx context.Context, channel, topic, id string, body []byte) error {
	ch, ok := l.channels[channel]
	if !ok {
		return ErrChannelNotFound
	}

	if ch.Access != nil && !ch.Access.CanWrite(l.SelfDID) {
		return ErrAccessDenied
	}
	if err := l.checkPerm(ctx, ch, l.SelfDID, permWrite); err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	rec := &SharedRecord{
		ID:        id,
		Channel:   channel,
		Topic:     topic,
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
// Returns nil if the record is expired.
func (l *GroupShareLayer) Get(ctx context.Context, channel, id string) (*SharedRecord, error) {
	rec, err := l.Storage.GetShared(ctx, channel, id)
	if err != nil || rec == nil {
		return nil, err
	}
	if l.IsExpired(rec, time.Now().UnixMilli()) {
		return nil, nil
	}
	return rec, nil
}

// Delete marks a shared record as deleted and broadcasts to group members.
func (l *GroupShareLayer) Delete(ctx context.Context, channel, topic, id string) error {
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
		Topic:     topic,
		GroupID:   ch.GroupID,
		DID:       l.SelfDID,
		Timestamp: now,
		Deleted:   true,
	}

	return l.broadcast(ctx, ch, rec)
}

// List returns all non-expired shared records in a channel.
func (l *GroupShareLayer) List(ctx context.Context, channel string) ([]*SharedRecord, error) {
	recs, err := l.Storage.ListByChannel(ctx, channel)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	var result []*SharedRecord
	for _, r := range recs {
		if !l.IsExpired(r, now) {
			result = append(result, r)
		}
	}
	return result, nil
}

// Dump returns all non-expired shared records for a group across all channels.
// The infra layer does not check permissions — the app layer is responsible.
func (l *GroupShareLayer) Dump(ctx context.Context, groupID string) ([]*SharedRecord, error) {
	recs, err := l.Storage.ListByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	var result []*SharedRecord
	for _, r := range recs {
		if !l.IsExpired(r, now) {
			result = append(result, r)
		}
	}
	return result, nil
}

// Restore applies shared records using last-write-wins by timestamp.
// Returns the number of records actually applied (newer than existing).
// The infra layer does not check permissions — the app layer is responsible.
func (l *GroupShareLayer) Restore(ctx context.Context, records []*SharedRecord) (int, error) {
	applied := 0
	for _, rec := range records {
		existing, err := l.Storage.GetTimestamp(ctx, rec.Channel, rec.ID)
		if err != nil {
			return applied, err
		}
		if rec.Timestamp <= existing {
			continue // skip older
		}
		if err := l.Storage.PutShared(ctx, rec); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, nil
}

// Purge physically removes expired records from a channel's storage.
// Returns the number of records purged.
// If the channel has no retention configured (permanent), returns 0.
func (l *GroupShareLayer) Purge(ctx context.Context, channel string) (int, error) {
	ch, ok := l.channels[channel]
	if !ok {
		return 0, ErrChannelNotFound
	}
	if ch.Retention == 0 {
		return 0, nil
	}

	before := time.Now().UnixMilli() - ch.Retention.Milliseconds()
	return l.Storage.DeleteExpired(ctx, channel, before)
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
	if err := l.checkPerm(ctx, ch, rec.DID, permRead); err != nil {
		return err
	}

	// Validate schema (only for non-delete)
	if !rec.Deleted && ch.Schema != nil {
		if err := ch.Schema.Validate(rec.Body); err != nil {
			return ErrSchemaValidation
		}
	}

	// Reject records that are already expired
	if l.IsExpired(&rec, time.Now().UnixMilli()) {
		return nil // silently drop expired incoming records
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

// broadcast sends a shared record to group members, filtering by subscription.
// If RemoteSubs is nil, sends to all members (backward compat).
// If a member's subscription is unknown (nil), sends to them (safe default).
// If a member's subscription is set but doesn't match the topic, skips them.
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

	// Filter members by subscription if RemoteSubs is available
	if l.RemoteSubs != nil {
		var filtered []string
		for _, did := range members {
			topics, err := l.RemoteSubs.GetSubscription(did, rec.Channel)
			if err != nil || topics == nil {
				// Unknown subscription → include (safe default)
				filtered = append(filtered, did)
				continue
			}
			if TopicMatches(topics, rec.Topic) {
				filtered = append(filtered, did)
			}
		}
		members = filtered
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

// permOp identifies which permission field to check.
type permOp int

const (
	permRead permOp = iota
	permWrite
	permDelete
)

// checkPerm verifies role-based permission for the given DID on the channel.
// Returns nil if no role DAG or no Perms are configured (allow all).
func (l *GroupShareLayer) checkPerm(ctx context.Context, ch *Channel, did string, op permOp) error {
	if l.roleDAG == nil || ch.Perms == nil || l.memberRoleResolver == nil {
		return nil
	}
	memberRole, err := l.memberRoleResolver.MemberRole(ctx, ch.GroupID, did)
	if err != nil {
		return err
	}
	var required string
	switch op {
	case permRead:
		required = ch.Perms.Read
	case permWrite:
		required = ch.Perms.Write
	case permDelete:
		required = ch.Perms.Delete
	}
	if required == "" {
		return nil
	}
	if !permission.Check(l.roleDAG, memberRole, required) {
		return ErrAccessDenied
	}
	return nil
}
