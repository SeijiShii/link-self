// Package integration runs local multi-node tests for LinkSelf Phase 1.
package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/SeijiShii/link-self/core/internal/did"
	"github.com/SeijiShii/link-self/core/internal/node"
	"github.com/libp2p/go-libp2p/core/peer"
)

const testTimeout = 30 * time.Second

// TestDIDGenerationConsistency: same private key yields same DID.
func TestDIDGenerationConsistency(t *testing.T) {
	id1, err := did.Generate()
	if err != nil {
		t.Fatal(err)
	}
	id2, err := did.FromPrivKey(id1.PrivKey)
	if err != nil {
		t.Fatal(err)
	}
	if id1.DID != id2.DID {
		t.Errorf("same key produced different DIDs: %q vs %q", id1.DID, id2.DID)
	}
}

// TestDHTProvideFind: Node B bootstraps to A, then finds A's AddrInfo via public DHT FindPeer(DIDToPeerID).
func TestDHTProvideFind(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	nA, err := node.New(ctx, node.Config{
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
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers: []peer.AddrInfo{infoA},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer nB.Close()
	if err := nB.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := nB.Host.Connect(ctx, infoA); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)

	pid, err := did.DIDToPeerID(nA.Identity.DID)
	if err != nil {
		t.Fatalf("DIDToPeerID: %v", err)
	}
	found, err := nB.DHT.FindPeer(ctx, pid)
	if err != nil {
		t.Fatalf("FindPeer: %v", err)
	}
	if found.ID != nA.Host.ID() {
		t.Errorf("found wrong peer: %s vs %s", found.ID, nA.Host.ID())
	}
}

// TestConnectAuth: B connects to A by DID and auth succeeds.
func TestConnectAuth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	nA, err := node.New(ctx, node.Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}})
	if err != nil {
		t.Fatal(err)
	}
	defer nA.Close()
	if err := nA.Start(ctx); err != nil {
		t.Fatal(err)
	}
	infoA := peer.AddrInfo{ID: nA.Host.ID(), Addrs: nA.Host.Addrs()}

	nB, err := node.New(ctx, node.Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers: []peer.AddrInfo{infoA},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer nB.Close()
	if err := nB.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := nB.Host.Connect(ctx, infoA); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)

	stream, err := nB.Connect(ctx, nA.Identity.DID)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	stream.Close()
}

// TestMessageOnline: A and B online; B sends message to A, A receives.
func TestMessageOnline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	nA, err := node.New(ctx, node.Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}})
	if err != nil {
		t.Fatal(err)
	}
	defer nA.Close()
	var received []byte
	var recvMu sync.Mutex
	nA.SetOnMessage(func(peerDID string, payload []byte) {
		recvMu.Lock()
		received = payload
		recvMu.Unlock()
	})
	if err := nA.Start(ctx); err != nil {
		t.Fatal(err)
	}
	infoA := peer.AddrInfo{ID: nA.Host.ID(), Addrs: nA.Host.Addrs()}

	nB, err := node.New(ctx, node.Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers: []peer.AddrInfo{infoA},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer nB.Close()
	if err := nB.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := nB.Host.Connect(ctx, infoA); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)

	msg := []byte("hello from B")
	if err := nB.SendMessage(ctx, nA.Identity.DID, msg); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	time.Sleep(1 * time.Second)
	recvMu.Lock()
	got := received
	recvMu.Unlock()
	if string(got) != string(msg) {
		t.Errorf("received %q, want %q", got, msg)
	}
}

// TestStoreAndForward: B sends to A while A is offline; A comes online; B delivers via Connect (flush).
func TestStoreAndForward(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	idA, err := did.Generate()
	if err != nil {
		t.Fatal(err)
	}

	nB, err := node.New(ctx, node.Config{ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"}})
	if err != nil {
		t.Fatal(err)
	}
	defer nB.Close()
	if err := nB.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := nB.SendMessage(ctx, idA.DID, []byte("queued hello")); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if nB.StoreForward.PendingCount(idA.DID) != 1 {
		t.Errorf("expected 1 pending, got %d", nB.StoreForward.PendingCount(idA.DID))
	}

	nA, err := node.New(ctx, node.Config{
		Identity:    idA,
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer nA.Close()
	var received []byte
	var recvMu sync.Mutex
	nA.SetOnMessage(func(peerDID string, payload []byte) {
		recvMu.Lock()
		received = payload
		recvMu.Unlock()
	})
	if err := nA.Start(ctx); err != nil {
		t.Fatal(err)
	}
	infoA := peer.AddrInfo{ID: nA.Host.ID(), Addrs: nA.Host.Addrs()}
	if err := nB.Host.Connect(ctx, infoA); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)

	// B connects to A by DID; Connect flushes the queue for idA
	stream, err := nB.Connect(ctx, idA.DID)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	stream.Close()
	time.Sleep(500 * time.Millisecond)
	recvMu.Lock()
	got := received
	recvMu.Unlock()
	if string(got) != "queued hello" {
		t.Errorf("received %q, want queued hello", got)
	}
}
