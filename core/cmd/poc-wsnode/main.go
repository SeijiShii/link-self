// poc-wsnode is a throwaway harness for the M1 browser-interop PoC
// (docs/spec/browser-pwa-support.md §7). It starts a LinkSelf node listening
// on WebSocket, prints its address as JSON on stdout, and echoes every
// received message back to the sender with an "echo:" prefix.
//
// Usage:
//
//	go run ./cmd/poc-wsnode            # direct-dial node (ws)
//	go run ./cmd/poc-wsnode -relay     # also start a relay + leaf behind it
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/SeijiShii/link-self/core/internal/auth"
	"github.com/SeijiShii/link-self/core/internal/did"
	"github.com/SeijiShii/link-self/core/internal/node"
	"github.com/libp2p/go-libp2p/core/peer"
)

type nodeInfo struct {
	Role       string `json:"role"` // "echo" | "relay" | "leaf"
	DID        string `json:"did"`
	PeerID     string `json:"peerID"`
	WSAddr     string `json:"wsAddr,omitempty"`     // dialable ws multiaddr with /p2p/
	CircuitVia string `json:"circuitVia,omitempty"` // relay circuit addr with /p2p/ (leaf only)
}

func wsAddrOf(n *node.Node) string {
	for _, a := range n.Host.Addrs() {
		if strings.Contains(a.String(), "/ws") {
			return a.String() + "/p2p/" + n.Host.ID().String()
		}
	}
	return ""
}

// startEchoNode starts a node that echoes every received message back to the
// sender with a role-specific prefix. The sender's transport PeerID is derived
// deterministically from its DID (did:key ⇔ Ed25519 pubkey ⇔ PeerID).
func startEchoNode(ctx context.Context, cfg node.Config, role string) (*node.Node, error) {
	n, err := node.New(ctx, cfg)
	if err != nil {
		return nil, err
	}
	n.SetOnMessage(func(peerDID string, payload []byte) {
		fmt.Fprintf(os.Stderr, "[%s] received from %s: %q\n", role, peerDID, payload)
		pid, err := did.DIDToPeerID(peerDID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%s] DIDToPeerID: %v\n", role, err)
			return
		}
		reply := append([]byte(role+":"), payload...)
		// "auth-me": run the LinkSelf auth initiator toward the sender, so the
		// remote (js) side can prove its responder implementation.
		if string(payload) == "auth-me" {
			if s, err := auth.RunInitiator(ctx, n.Host, pid, peerDID); err != nil {
				reply = []byte("auth-fail:" + err.Error())
			} else {
				s.Close()
				reply = []byte("auth-ok")
			}
		}
		if err := n.SendToPeerID(ctx, pid, reply); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] send back: %v\n", role, err)
		}
	})
	if err := n.Start(ctx); err != nil {
		n.Close()
		return nil, err
	}
	return n, nil
}

func main() {
	relayMode := flag.Bool("relay", false, "also start a relay and a leaf node reachable only via the relay")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Echo node: direct ws dial target.
	echo, err := startEchoNode(ctx, node.Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0/ws"},
	}, "echo")
	if err != nil {
		fmt.Fprintln(os.Stderr, "start echo node:", err)
		os.Exit(1)
	}
	defer echo.Close()

	out := json.NewEncoder(os.Stdout)
	_ = out.Encode(nodeInfo{
		Role:   "echo",
		DID:    echo.Identity.DID,
		PeerID: echo.Host.ID().String(),
		WSAddr: wsAddrOf(echo),
	})

	if *relayMode {
		relay, err := startEchoNode(ctx, node.Config{
			ListenAddrs:        []string{"/ip4/127.0.0.1/tcp/0/ws"},
			EnableRelayService: true,
			ForceReachability:  "public",
		}, "relay")
		if err != nil {
			fmt.Fprintln(os.Stderr, "start relay node:", err)
			os.Exit(1)
		}
		defer relay.Close()
		_ = out.Encode(nodeInfo{
			Role:   "relay",
			DID:    relay.Identity.DID,
			PeerID: relay.Host.ID().String(),
			WSAddr: wsAddrOf(relay),
		})

		leaf, err := startEchoNode(ctx, node.Config{
			ListenAddrs:       []string{"/ip4/127.0.0.1/tcp/0"},
			StaticRelays:      []peer.AddrInfo{{ID: relay.Host.ID(), Addrs: relay.Host.Addrs()}},
			ForceReachability: "private",
		}, "leaf")
		if err != nil {
			fmt.Fprintln(os.Stderr, "start leaf node:", err)
			os.Exit(1)
		}
		defer leaf.Close()
		_ = out.Encode(nodeInfo{
			Role:       "leaf",
			DID:        leaf.Identity.DID,
			PeerID:     leaf.Host.ID().String(),
			CircuitVia: wsAddrOf(relay) + "/p2p-circuit/p2p/" + leaf.Host.ID().String(),
		})
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
}
