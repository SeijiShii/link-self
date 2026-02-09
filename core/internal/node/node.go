// Package node assembles a LinkSelf node: libp2p Host + DHT (DID Provide/Find) + auth.
package node

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/SeijiShii/link-self/core/internal/auth"
	"github.com/SeijiShii/link-self/core/internal/did"
	"github.com/SeijiShii/link-self/core/internal/dht"
	"github.com/SeijiShii/link-self/core/internal/storeforward"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
)

// Message protocol for LinkSelf: length-prefixed payload.
const MessageProtocol = "/linkself/msg/1.0.0"

const defaultDialTimeout = 30 * time.Second

// Node is a LinkSelf node: identity, libp2p host, DHT, auth handler, and store-forward.
type Node struct {
	Identity     *did.Identity
	Host         host.Host
	DHT          *kaddht.IpfsDHT
	StoreForward *storeforward.StoreForward
	usePublicDHT bool
	mu           sync.Mutex
	onMessage    func(peerDID string, payload []byte)
}

// Config holds options for creating a Node.
type Config struct {
	Identity       *did.Identity
	ListenAddrs    []string
	BootstrapPeers []peer.AddrInfo
	// UsePublicDHT uses /ipfs protocol and FindPeer(DIDToPeerID) instead of PutDID/FindDID.
	// Set true to join the public DHT (bootstrap.libp2p.io) and discover peers by PeerID.
	UsePublicDHT bool
}

// New creates a new Node (Host + DHT with linkself validator). Call Start to register in DHT and set auth handler.
func New(ctx context.Context, cfg Config) (*Node, error) {
	if cfg.Identity == nil {
		id, err := did.Generate()
		if err != nil {
			return nil, err
		}
		cfg.Identity = id
	}
	lopts := []libp2p.Option{
		libp2p.Identity(cfg.Identity.PrivKey),
	}
	if len(cfg.ListenAddrs) > 0 {
		lopts = append(lopts, libp2p.ListenAddrStrings(cfg.ListenAddrs...))
	}
	h, err := libp2p.New(lopts...)
	if err != nil {
		return nil, fmt.Errorf("create host: %w", err)
	}
	dhtOpts := []kaddht.Option{
		kaddht.Mode(kaddht.ModeServer),
	}
	if cfg.UsePublicDHT {
		dhtOpts = append(dhtOpts, kaddht.ProtocolPrefix(protocol.ID("/ipfs")))
		// No custom Validator: use default pk/ipns so public nodes accept us.
	} else {
		dhtOpts = append(dhtOpts, kaddht.ProtocolPrefix("/linkself"), kaddht.Validator(dht.LinkselfValidator()))
	}
	if len(cfg.BootstrapPeers) > 0 {
		dhtOpts = append(dhtOpts, kaddht.BootstrapPeers(cfg.BootstrapPeers...))
	}
	kdht, err := kaddht.New(ctx, h, dhtOpts...)
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("create DHT: %w", err)
	}
	return &Node{
		Identity:     cfg.Identity,
		Host:         h,
		DHT:          kdht,
		StoreForward: storeforward.New(),
		usePublicDHT: cfg.UsePublicDHT,
	}, nil
}

// Start registers this node in the DHT (when not UsePublicDHT) and sets the auth stream handler.
// When UsePublicDHT, no PutDID; Bootstrap runs in background so FindPeer can discover us.
func (n *Node) Start(ctx context.Context) error {
	if !n.usePublicDHT {
		info := peer.AddrInfo{
			ID:    n.Host.ID(),
			Addrs: n.Host.Addrs(),
		}
		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				if err := dht.PutDID(context.Background(), n.DHT, n.Identity.DID, info); err == nil {
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	} else {
		go func() {
			_ = n.DHT.Bootstrap(context.Background())
		}()
	}
	n.Host.SetStreamHandler(protocol.ID(auth.ProtocolID), func(s network.Stream) {
		remoteID := s.Conn().RemotePeer()
		_ = auth.ChallengeResponse(s, n.Identity.PrivKey)
		if pub := n.Host.Peerstore().PubKey(remoteID); pub != nil {
			if peerDID, err := did.PubKeyToDID(pub); err == nil {
				_, _ = n.StoreForward.FlushForDID(peerDID, remoteID, n.sendMessageToPeer)
			}
		}
	})
	n.Host.SetStreamHandler(protocol.ID(MessageProtocol), func(s network.Stream) {
		defer s.Close()
		var payloadLen uint32
		if err := binary.Read(s, binary.BigEndian, &payloadLen); err != nil {
			return
		}
		if payloadLen > 1<<20 {
			return
		}
		payload := make([]byte, payloadLen)
		if _, err := io.ReadFull(s, payload); err != nil {
			return
		}
		remoteID := s.Conn().RemotePeer()
		if pub := n.Host.Peerstore().PubKey(remoteID); pub != nil {
			if peerDID, err := did.PubKeyToDID(pub); err == nil && n.onMessage != nil {
				n.onMessage(peerDID, payload)
			}
		}
	})
	return nil
}

func (n *Node) sendMessageToPeer(pid peer.ID, payload []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultDialTimeout)
	defer cancel()
	s, err := n.Host.NewStream(ctx, pid, protocol.ID(MessageProtocol))
	if err != nil {
		return err
	}
	defer s.Close()
	if err := binary.Write(s, binary.BigEndian, uint32(len(payload))); err != nil {
		return err
	}
	_, err = s.Write(payload)
	return err
}

// SetOnMessage sets a callback invoked when a message is received (peerDID, payload).
func (n *Node) SetOnMessage(fn func(peerDID string, payload []byte)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.onMessage = fn
}

// SendToGroup sends a message to each member DID (excluding self). Uses SendMessage per DID;
// offline peers are queued by store-and-forward.
func (n *Node) SendToGroup(ctx context.Context, memberDIDs []string, payload []byte) error {
	myDID := n.Identity.DID
	for _, did := range memberDIDs {
		if did == myDID {
			continue
		}
		_ = n.SendMessage(ctx, did, payload)
	}
	return nil
}

// findPeerByDID returns AddrInfo for the given DID (DHT FindDID or public DHT FindPeer).
func (n *Node) findPeerByDID(ctx context.Context, peerDID string) (peer.AddrInfo, error) {
	if peerDID == "" {
		return peer.AddrInfo{}, fmt.Errorf("peerDID must not be empty")
	}
	if n.usePublicDHT {
		pid, err := did.DIDToPeerID(peerDID)
		if err != nil {
			return peer.AddrInfo{}, fmt.Errorf("DID to peer ID: %w", err)
		}
		return n.DHT.FindPeer(ctx, pid)
	}
	return dht.FindDID(ctx, n.DHT, peerDID)
}

// SendMessage sends a message to the peer with the given DID. If the peer is not reachable,
// the message is queued (store-and-forward) and sent when the peer comes online.
func (n *Node) SendMessage(ctx context.Context, peerDID string, payload []byte) error {
	info, err := n.findPeerByDID(ctx, peerDID)
	if err != nil {
		n.StoreForward.Queue(peerDID, payload)
		return nil
	}
	if err := n.Host.Connect(ctx, info); err != nil {
		n.StoreForward.Queue(peerDID, payload)
		return nil
	}
	authStream, err := auth.RunInitiator(ctx, n.Host, info.ID, peerDID)
	if err != nil {
		n.StoreForward.Queue(peerDID, payload)
		return nil
	}
	authStream.Close()
	return n.sendMessageToPeer(info.ID, payload)
}

// Connect finds the peer by DID in the DHT, dials, and runs challenge-response auth.
// Returns an authenticated stream (caller should close it when done).
// Also flushes any store-and-forward messages for this peer.
func (n *Node) Connect(ctx context.Context, peerDID string) (network.Stream, error) {
	info, err := n.findPeerByDID(ctx, peerDID)
	if err != nil {
		return nil, fmt.Errorf("find DID: %w", err)
	}
	if err := n.Host.Connect(ctx, info); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	stream, err := auth.RunInitiator(ctx, n.Host, info.ID, peerDID)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	_, _ = n.StoreForward.FlushForDID(peerDID, info.ID, n.sendMessageToPeer)
	return stream, nil
}

// Close shuts down the DHT and host.
func (n *Node) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	var errs []error
	if err := n.DHT.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := n.Host.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
