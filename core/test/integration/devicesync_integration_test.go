package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/SeijiShii/link-self/core/internal/devicesync"
	"github.com/SeijiShii/link-self/core/internal/envelope"
	"github.com/SeijiShii/link-self/core/internal/node"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestDeviceSync_TwoDevices: Two nodes with different transport keys but same DID.
// Node A puts a record; Node B receives it via DeviceSync replication.
func TestDeviceSync_TwoDevices(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Both devices share the same DID (but have different transport keys).
	sharedDID := "did:key:shared-for-test"

	// Create two nodes with their own transport identities.
	nA, err := node.New(ctx, node.Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer nA.Close()

	infoA := peer.AddrInfo{ID: nA.Host.ID(), Addrs: nA.Host.Addrs()}

	nB, err := node.New(ctx, node.Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers: []peer.AddrInfo{infoA},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer nB.Close()

	storageA := devicesync.NewMemStorage()
	storageB := devicesync.NewMemStorage()

	// Wire ReplicationEngine for Node A.
	// SendFunc: wrap ChangeEntry in devicesync envelope, send to peer's transport PeerID.
	engineA := &devicesync.ReplicationEngine{
		Storage: storageA,
		SelfDID: sharedDID,
		Send: func(ctx context.Context, peerID string, payload []byte) error {
			wrapped, err := envelope.Wrap(envelope.TypeDeviceSync, payload)
			if err != nil {
				return err
			}
			pid, err := peer.Decode(peerID)
			if err != nil {
				return err
			}
			return nA.SendToPeerID(ctx, pid, wrapped)
		},
		Peers: func(ctx context.Context) ([]string, error) {
			return []string{nB.Host.ID().String()}, nil
		},
	}

	engineB := &devicesync.ReplicationEngine{
		Storage: storageB,
		SelfDID: sharedDID,
		Send: func(ctx context.Context, peerID string, payload []byte) error {
			wrapped, err := envelope.Wrap(envelope.TypeDeviceSync, payload)
			if err != nil {
				return err
			}
			pid, err := peer.Decode(peerID)
			if err != nil {
				return err
			}
			return nB.SendToPeerID(ctx, pid, wrapped)
		},
		Peers: func(ctx context.Context) ([]string, error) {
			return []string{nA.Host.ID().String()}, nil
		},
	}

	// Wire OnDeviceSync handlers.
	var muB sync.Mutex
	nB.SetOnDeviceSync(func(peerDID string, payload []byte) {
		var entry devicesync.ChangeEntry
		if err := json.Unmarshal(payload, &entry); err != nil {
			return
		}
		muB.Lock()
		defer muB.Unlock()
		_ = engineB.HandleIncoming(ctx, &entry)
	})

	var muA sync.Mutex
	nA.SetOnDeviceSync(func(peerDID string, payload []byte) {
		var entry devicesync.ChangeEntry
		if err := json.Unmarshal(payload, &entry); err != nil {
			return
		}
		muA.Lock()
		defer muA.Unlock()
		_ = engineA.HandleIncoming(ctx, &entry)
	})

	// Start nodes and connect.
	if err := nA.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := nB.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := nB.Host.Connect(ctx, infoA); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)

	// A puts a record.
	if err := engineA.Put(ctx, "contacts", "alice", []byte(`{"name":"Alice"}`)); err != nil {
		t.Fatalf("engineA.Put: %v", err)
	}

	// Wait for replication.
	time.Sleep(2 * time.Second)

	// Verify B received the record.
	muB.Lock()
	rec, err := storageB.Get(ctx, "contacts", "alice")
	muB.Unlock()
	if err != nil {
		t.Fatalf("storageB.Get: %v", err)
	}
	if rec == nil {
		t.Fatal("Node B should have received the record from Node A")
	}
	if string(rec.Body) != `{"name":"Alice"}` {
		t.Errorf("body = %q, want {\"name\":\"Alice\"}", rec.Body)
	}

	// B puts a record back.
	if err := engineB.Put(ctx, "contacts", "bob", []byte(`{"name":"Bob"}`)); err != nil {
		t.Fatalf("engineB.Put: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Verify A received B's record.
	muA.Lock()
	rec2, err := storageA.Get(ctx, "contacts", "bob")
	muA.Unlock()
	if err != nil {
		t.Fatalf("storageA.Get: %v", err)
	}
	if rec2 == nil {
		t.Fatal("Node A should have received the record from Node B")
	}
	if string(rec2.Body) != `{"name":"Bob"}` {
		t.Errorf("body = %q, want {\"name\":\"Bob\"}", rec2.Body)
	}
}
