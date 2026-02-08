// Package integration: integration test for node + group + syncdb.
package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/SeijiShii/link-self/core/internal/did"
	"github.com/SeijiShii/link-self/core/internal/group"
	"github.com/SeijiShii/link-self/core/internal/node"
	"github.com/SeijiShii/link-self/core/internal/syncdb"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestSyncDB_PutOnA_ReceivedOnB: A puts a record for a shared group; B receives via SetOnMessage and applies to storage.
func TestSyncDB_PutOnA_ReceivedOnB(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	idA, _ := did.Generate()
	idB, _ := did.Generate()

	groupStore := group.NewMemStore()
	groupID, err := groupStore.CreateGroup([]string{idA.DID, idB.DID}, []string{idA.DID})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	storageA := syncdb.NewMemStorage()
	storageB := syncdb.NewMemStorage()

	nA, err := node.New(ctx, node.Config{
		Identity:    idA,
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer nA.Close()
	if err := nA.Start(ctx); err != nil {
		t.Fatal(err)
	}
	infoA := peer.AddrInfo{ID: nA.Host.ID(), Addrs: nA.Host.Addrs()}

	nB, err := node.New(ctx, node.Config{
		Identity:       idB,
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers: []peer.AddrInfo{infoA},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer nB.Close()

	resolverB := &syncdb.GroupStoreResolver{Store: groupStore, SelfDID: idB.DID}
	syncB := syncdb.NewSyncLayer(storageB, resolverB, nB.SendToGroup, idB.DID)
	var recvMu sync.Mutex
	nB.SetOnMessage(func(peerDID string, payload []byte) {
		recvMu.Lock()
		_ = syncB.HandleIncoming(ctx, payload)
		recvMu.Unlock()
	})
	if err := nB.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := nB.Host.Connect(ctx, infoA); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)

	resolverA := &syncdb.GroupStoreResolver{Store: groupStore, SelfDID: idA.DID}
	syncA := syncdb.NewSyncLayer(storageA, resolverA, nA.SendToGroup, idA.DID)

	record, err := syncA.Put(ctx, groupID, []byte("hello from A"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if record == nil || record.ID == "" {
		t.Fatal("Put should return record with ID")
	}

	time.Sleep(2 * time.Second)

	got, err := storageB.Get(ctx, record.ID)
	if err != nil {
		t.Fatalf("storageB.Get: %v", err)
	}
	if got == nil {
		t.Fatal("B should have received and applied the record")
	}
	if string(got.Body) != "hello from A" {
		t.Errorf("B storage body: got %q", got.Body)
	}
}
