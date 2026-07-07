// Browser-facing transport tests (WebSocket / Circuit Relay v2).
// Browser peers (js-libp2p) can only dial WebSocket / WebTransport / WebRTC,
// so always-on Go nodes must be able to listen on those transports.
// See docs/spec/browser-pwa-support.md.
package integration

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SeijiShii/link-self/core/internal/node"
	"github.com/libp2p/go-libp2p/core/peer"
)

// wsAddrOf returns the first /ws multiaddr of the node, or "" if none.
func wsAddrOf(n *node.Node) string {
	for _, a := range n.Host.Addrs() {
		if strings.Contains(a.String(), "/ws") {
			return a.String()
		}
	}
	return ""
}

// TestWebSocketListenAndConnect: A listens on WebSocket only; B dials A over ws,
// auth succeeds and a message is delivered. Proves the Go node can serve
// browser-style WebSocket connections (11_LinkSelf拡張要望 §3.1.2).
func TestWebSocketListenAndConnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	nA, err := node.New(ctx, node.Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0/ws"},
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

	wsAddr := wsAddrOf(nA)
	if wsAddr == "" {
		t.Fatalf("node A has no /ws listen addr; addrs = %v", nA.Host.Addrs())
	}

	nB, err := node.New(ctx, node.Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer nB.Close()
	if err := nB.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Dial A explicitly via its ws multiaddr (A listens on ws only, so the
	// connection cannot fall back to raw TCP).
	stream, err := nB.ConnectToAddr(ctx, nA.Identity.DID, wsAddr+"/p2p/"+nA.Host.ID().String())
	if err != nil {
		t.Fatalf("ConnectToAddr over ws: %v", err)
	}
	stream.Close()

	msg := []byte("hello over websocket")
	if err := nB.SendToPeerID(ctx, nA.Host.ID(), msg); err != nil {
		t.Fatalf("SendToPeerID: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		recvMu.Lock()
		got := received
		recvMu.Unlock()
		if string(got) == string(msg) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("message not received over ws: got %q, want %q", got, msg)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// The underlying connection must actually be WebSocket.
	conns := nB.Host.Network().ConnsToPeer(nA.Host.ID())
	if len(conns) == 0 {
		t.Fatal("no connection to A")
	}
	for _, c := range conns {
		if !strings.Contains(c.RemoteMultiaddr().String(), "/ws") {
			t.Errorf("connection is not websocket: %s", c.RemoteMultiaddr())
		}
	}
}

// TestCircuitRelayConnect: R serves Circuit Relay v2; A (unreachable, static relay = R)
// obtains a /p2p-circuit address; B dials A through R and a message is delivered.
// Models a browser-style leaf that cannot accept inbound connections
// (11_LinkSelf拡張要望 §3.1.3).
func TestCircuitRelayConnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	nR, err := node.New(ctx, node.Config{
		ListenAddrs:        []string{"/ip4/127.0.0.1/tcp/0", "/ip4/127.0.0.1/tcp/0/ws"},
		EnableRelayService: true,
		// The relay service only starts once the host believes it is publicly
		// reachable; on loopback AutoNAT never concludes that, so force it.
		ForceReachability: "public",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer nR.Close()
	if err := nR.Start(ctx); err != nil {
		t.Fatal(err)
	}
	infoR := peer.AddrInfo{ID: nR.Host.ID(), Addrs: nR.Host.Addrs()}

	nA, err := node.New(ctx, node.Config{
		ListenAddrs:       []string{"/ip4/127.0.0.1/tcp/0"},
		StaticRelays:      []peer.AddrInfo{infoR},
		ForceReachability: "private", // leaf: force autorelay to reserve a relay slot
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

	nB, err := node.New(ctx, node.Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer nB.Close()
	if err := nB.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Build the circuit addr manually: <R-addr>/p2p/<R>/p2p-circuit/p2p/<A>.
	// (autorelay only *advertises* circuit addrs whose relay addr is public —
	// see cleanupAddressSet — so on loopback A's Host.Addrs() stays empty of
	// them; with a real public relay they appear automatically.)
	circuitAddr := infoR.Addrs[0].String() + "/p2p/" + nR.Host.ID().String() +
		"/p2p-circuit/p2p/" + nA.Host.ID().String()

	// A's relay reservation is made asynchronously; retry until the relay
	// accepts the CONNECT.
	var stream interface{ Close() error }
	deadline := time.Now().Add(15 * time.Second)
	for {
		s, err := nB.ConnectToAddr(ctx, nA.Identity.DID, circuitAddr)
		if err == nil {
			stream = s
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("ConnectToAddr via relay: %v", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
	stream.Close()

	msg := []byte("hello via relay")
	if err := nB.SendToPeerID(ctx, nA.Host.ID(), msg); err != nil {
		t.Fatalf("SendToPeerID via relay: %v", err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		recvMu.Lock()
		got := received
		recvMu.Unlock()
		if string(got) == string(msg) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("message not received via relay: got %q, want %q", got, msg)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
