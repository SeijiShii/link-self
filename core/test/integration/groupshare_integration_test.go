package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/SeijiShii/link-self/core/internal/did"
	"github.com/SeijiShii/link-self/core/internal/envelope"
	"github.com/SeijiShii/link-self/core/internal/group"
	"github.com/SeijiShii/link-self/core/internal/groupshare"
	"github.com/SeijiShii/link-self/core/internal/node"
	"github.com/libp2p/go-libp2p/core/peer"
)

// groupMemberResolver adapts group.Store to groupshare.MemberResolver.
type groupMemberResolver struct {
	store   group.Store
	selfDID string
}

func (r *groupMemberResolver) MemberDIDsForGroup(ctx context.Context, groupID string) ([]string, error) {
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

// TestGroupShare_ChannelBroadcast: Alice and Bob in a group. Alice puts a shared record;
// Bob receives it via GroupShare channel.
func TestGroupShare_ChannelBroadcast(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	idAlice, _ := did.Generate()
	idBob, _ := did.Generate()

	// Create a group with both members.
	groupStore := group.NewMemStore()
	groupID, err := groupStore.CreateGroup([]string{idAlice.DID, idBob.DID}, []string{idAlice.DID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	// Create nodes.
	nAlice, err := node.New(ctx, node.Config{
		Identity:    idAlice,
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer nAlice.Close()

	infoAlice := peer.AddrInfo{ID: nAlice.Host.ID(), Addrs: nAlice.Host.Addrs()}

	nBob, err := node.New(ctx, node.Config{
		Identity:       idBob,
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers: []peer.AddrInfo{infoAlice},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer nBob.Close()

	// Wire GroupShareLayer for Alice.
	storageAlice := groupshare.NewMemSharedStorage()
	resolverAlice := &groupMemberResolver{store: groupStore, selfDID: idAlice.DID}
	layerAlice := groupshare.NewGroupShareLayer(
		storageAlice,
		resolverAlice,
		func(ctx context.Context, memberDIDs []string, payload []byte) error {
			wrapped, err := envelope.Wrap(envelope.TypeGroupShare, payload)
			if err != nil {
				return err
			}
			return nAlice.SendToGroup(ctx, memberDIDs, wrapped)
		},
		idAlice.DID,
	)

	// Wire GroupShareLayer for Bob.
	storageBob := groupshare.NewMemSharedStorage()
	resolverBob := &groupMemberResolver{store: groupStore, selfDID: idBob.DID}
	layerBob := groupshare.NewGroupShareLayer(
		storageBob,
		resolverBob,
		func(ctx context.Context, memberDIDs []string, payload []byte) error {
			wrapped, err := envelope.Wrap(envelope.TypeGroupShare, payload)
			if err != nil {
				return err
			}
			return nBob.SendToGroup(ctx, memberDIDs, wrapped)
		},
		idBob.DID,
	)

	// Register the same channel on both sides.
	ch := &groupshare.Channel{Name: "notes", GroupID: groupID}
	if err := layerAlice.RegisterChannel(ch); err != nil {
		t.Fatal(err)
	}
	if err := layerBob.RegisterChannel(ch); err != nil {
		t.Fatal(err)
	}

	// Wire OnGroupShare handlers.
	var muBob sync.Mutex
	nBob.SetOnGroupShare(func(peerDID string, payload []byte) {
		muBob.Lock()
		defer muBob.Unlock()
		_ = layerBob.HandleIncoming(ctx, payload)
	})

	var muAlice sync.Mutex
	nAlice.SetOnGroupShare(func(peerDID string, payload []byte) {
		muAlice.Lock()
		defer muAlice.Unlock()
		_ = layerAlice.HandleIncoming(ctx, payload)
	})

	// Start nodes and connect.
	if err := nAlice.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := nBob.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := nBob.Host.Connect(ctx, infoAlice); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)

	// Alice puts a shared record.
	if err := layerAlice.Put(ctx, "notes", "note1", []byte(`{"text":"Hello from Alice"}`)); err != nil {
		t.Fatalf("layerAlice.Put: %v", err)
	}

	// Wait for delivery.
	time.Sleep(2 * time.Second)

	// Verify Bob received the record.
	muBob.Lock()
	rec, err := storageBob.GetShared(ctx, "notes", "note1")
	muBob.Unlock()
	if err != nil {
		t.Fatalf("storageBob.GetShared: %v", err)
	}
	if rec == nil {
		t.Fatal("Bob should have received the shared record from Alice")
	}
	if string(rec.Body) != `{"text":"Hello from Alice"}` {
		t.Errorf("body = %q, want {\"text\":\"Hello from Alice\"}", rec.Body)
	}
	if rec.DID != idAlice.DID {
		t.Errorf("writer DID = %q, want Alice's DID %q", rec.DID, idAlice.DID)
	}

	// Bob puts a record back.
	if err := layerBob.Put(ctx, "notes", "note2", []byte(`{"text":"Reply from Bob"}`)); err != nil {
		t.Fatalf("layerBob.Put: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Verify Alice received Bob's record.
	muAlice.Lock()
	rec2, err := storageAlice.GetShared(ctx, "notes", "note2")
	muAlice.Unlock()
	if err != nil {
		t.Fatalf("storageAlice.GetShared: %v", err)
	}
	if rec2 == nil {
		t.Fatal("Alice should have received the shared record from Bob")
	}
	if string(rec2.Body) != `{"text":"Reply from Bob"}` {
		t.Errorf("body = %q, want {\"text\":\"Reply from Bob\"}", rec2.Body)
	}
}
