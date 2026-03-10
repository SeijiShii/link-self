package linkself

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SeijiShii/link-self/core/internal/devicesync"
	"github.com/SeijiShii/link-self/core/internal/did"
	"github.com/SeijiShii/link-self/core/internal/envelope"
	"github.com/SeijiShii/link-self/core/internal/group"
	"github.com/SeijiShii/link-self/core/internal/groupshare"
	"github.com/SeijiShii/link-self/core/internal/node"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// client implements the Client interface using internal packages.
// This is the only place where internal packages are accessed.
// All other code should use the Client interface instead of this concrete type.
type client struct {
	node       *node.Node
	identity   *did.Identity
	deviceDB   *deviceDB
	groupShare *groupShareAPI
	groups     *groupAPI
}

// NewClient creates a new LinkSelf client.
// The returned client must be started with Start() before it can be used.
//
// NewClient は、新しいLinkSelfクライアントを作成します。
// 返されたクライアントは、使用する前にStart()で起動する必要があります。
//
// Example:
//
//	client := linkself.NewClient()
//	info, err := client.Start(ctx, config)
//	if err != nil {
//		return err
//	}
//	defer client.Stop(ctx)
func NewClient() Client {
	return &client{}
}

// Start starts a LinkSelf node with the given configuration.
// See Client.Start for detailed documentation.
func (c *client) Start(ctx context.Context, config Config) (*NodeInfo, error) {
	// Load or generate identity
	identityPath := config.IdentityPath
	if identityPath == "" {
		homeDir, _ := os.UserHomeDir()
		identityPath = filepath.Join(homeDir, ".linkself", "identity.json")
	}

	identity, err := loadOrGenerateIdentity(identityPath)
	if err != nil {
		return nil, fmt.Errorf("load identity: %w", err)
	}

	// Parse bootstrap peers
	var bootstrapPeers []peer.AddrInfo
	for _, addrStr := range config.BootstrapPeers {
		info, err := peer.AddrInfoFromString(addrStr)
		if err != nil {
			return nil, fmt.Errorf("parse bootstrap peer %q: %w", addrStr, err)
		}
		bootstrapPeers = append(bootstrapPeers, *info)
	}

	// Set default listen address if not provided
	listenAddrs := config.ListenAddrs
	if len(listenAddrs) == 0 {
		listenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}
	}

	// Create node
	cfg := node.Config{
		Identity:       identity,
		ListenAddrs:    listenAddrs,
		BootstrapPeers: bootstrapPeers,
	}

	n, err := node.New(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}

	// Start node
	if err := n.Start(ctx); err != nil {
		n.Close()
		return nil, fmt.Errorf("start node: %w", err)
	}

	c.node = n
	c.identity = identity

	// Wire DeviceSync layer (no peers initially — single device).
	dsStorage := devicesync.NewMemStorage()
	dsEngine := &devicesync.ReplicationEngine{
		Storage: dsStorage,
		SelfDID: identity.DID,
		Send: func(ctx context.Context, peerID string, payload []byte) error {
			wrapped, err := envelope.Wrap(envelope.TypeDeviceSync, payload)
			if err != nil {
				return err
			}
			pid, err := peer.Decode(peerID)
			if err != nil {
				return err
			}
			return n.SendToPeerID(ctx, pid, wrapped)
		},
	}
	c.deviceDB = &deviceDB{engine: dsEngine}

	// Wire Group layer.
	groupStore := group.NewMemStore()
	groupService := group.NewService(groupStore)
	c.groups = &groupAPI{
		service: groupService,
		store:   groupStore,
		selfDID: identity.DID,
	}

	// Wire GroupShare layer.
	gsStorage := groupshare.NewMemSharedStorage()
	gsResolver := &memberResolverAdapter{store: groupStore, selfDID: identity.DID}
	gsLayer := groupshare.NewGroupShareLayer(
		gsStorage,
		gsResolver,
		func(ctx context.Context, memberDIDs []string, payload []byte) error {
			wrapped, err := envelope.Wrap(envelope.TypeGroupShare, payload)
			if err != nil {
				return err
			}
			return n.SendToGroup(ctx, memberDIDs, wrapped)
		},
		identity.DID,
	)
	// Wire subscription stores: LocalSubs persisted via DeviceSync (auto-replicated to same-DID devices).
	gsLayer.LocalSubs = groupshare.NewDeviceSyncSubscriptionStore(dsEngine)
	gsLayer.RemoteSubs = groupshare.NewMemSubscriptionStore()
	gsLayer.SendSubAnnounce = func(ctx context.Context, memberDIDs []string, payload []byte) error {
		wrapped, err := envelope.Wrap(envelope.TypeSubAnnounce, payload)
		if err != nil {
			return err
		}
		return n.SendToGroup(ctx, memberDIDs, wrapped)
	}
	c.groupShare = &groupShareAPI{layer: gsLayer}

	// Wire incoming message handlers.
	n.SetOnGroupShare(func(peerDID string, payload []byte) {
		_ = gsLayer.HandleIncoming(context.Background(), payload)
	})
	n.SetOnSubAnnounce(func(peerDID string, payload []byte) {
		_ = gsLayer.HandleSubAnnouncement(peerDID, payload)
	})

	// Get listen address
	listenAddr := ""
	if len(n.Host.Addrs()) > 0 {
		listenAddr = fmt.Sprintf("%s/p2p/%s", n.Host.Addrs()[0].String(), n.Host.ID().String())
	}

	return &NodeInfo{
		DID:        identity.DID,
		ListenAddr: listenAddr,
	}, nil
}

// Stop stops the node and releases resources.
// See Client.Stop for detailed documentation.
func (c *client) Stop(ctx context.Context) error {
	if c.node != nil {
		err := c.node.Close()
		c.node = nil
		c.identity = nil
		return err
	}
	return nil
}

// GetMyDID returns the node's DID.
// See Client.GetMyDID for detailed documentation.
func (c *client) GetMyDID() string {
	if c.identity != nil {
		return c.identity.DID
	}
	return ""
}

// SendMessage sends a message to a peer.
// For 1-to-1 messaging, this creates a 2-person group internally.
// See Client.SendMessage for detailed documentation.
func (c *client) SendMessage(ctx context.Context, peerDID string, message string) error {
	if c.node == nil {
		return fmt.Errorf("node not started")
	}
	if c.identity == nil {
		return fmt.Errorf("identity not available")
	}

	// Send message (1-to-1 is a 2-person group)
	memberDIDs := []string{c.identity.DID, peerDID}
	return c.node.SendToGroup(ctx, memberDIDs, []byte(message))
}

// Connect connects to a peer and authenticates.
// By default finds the peer via the public DHT (FindPeer) and then dials and authenticates.
// If listenAddr is non-empty, connects directly to that address without DHT lookup (legacy).
func (c *client) Connect(ctx context.Context, peerDID string, listenAddr string) error {
	if c.node == nil {
		return fmt.Errorf("node not started")
	}
	var err error
	if listenAddr != "" {
		_, err = c.node.ConnectToAddr(ctx, peerDID, listenAddr)
	} else {
		_, err = c.node.Connect(ctx, peerDID)
	}
	return err
}

// SetOnMessage sets the message handler callback.
// See Client.SetOnMessage for detailed documentation.
func (c *client) SetOnMessage(handler MessageHandler) {
	if c.node != nil {
		c.node.SetOnMessage(handler)
	}
}

// DeviceDB returns the DeviceDB interface. Returns nil before Start.
func (c *client) DeviceDB() DeviceDB {
	if c.deviceDB == nil {
		return nil
	}
	return c.deviceDB
}

// GroupShare returns the GroupShareAPI interface. Returns nil before Start.
func (c *client) GroupShare() GroupShareAPI {
	if c.groupShare == nil {
		return nil
	}
	return c.groupShare
}

// Groups returns the GroupAPI interface. Returns nil before Start.
func (c *client) Groups() GroupAPI {
	if c.groups == nil {
		return nil
	}
	return c.groups
}

// loadOrGenerateIdentity loads an identity from file or generates a new one.
func loadOrGenerateIdentity(path string) (*did.Identity, error) {
	// Try to load existing identity
	if data, err := os.ReadFile(path); err == nil {
		var keyData struct {
			PrivKey []byte `json:"privKey"`
		}
		if err := json.Unmarshal(data, &keyData); err == nil {
			privKey, err := crypto.UnmarshalPrivateKey(keyData.PrivKey)
			if err == nil {
				return did.FromPrivKey(privKey)
			}
		}
	}

	// Generate new identity
	identity, err := did.Generate()
	if err != nil {
		return nil, fmt.Errorf("generate identity: %w", err)
	}

	// Save identity
	if err := saveIdentity(path, identity); err != nil {
		// Log error but don't fail
		fmt.Fprintf(os.Stderr, "Warning: failed to save identity: %v\n", err)
	}

	return identity, nil
}

// saveIdentity saves an identity to a file.
func saveIdentity(path string, identity *did.Identity) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Marshal private key
	privKeyBytes, err := crypto.MarshalPrivateKey(identity.PrivKey)
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}

	keyData := struct {
		PrivKey []byte `json:"privKey"`
	}{
		PrivKey: privKeyBytes,
	}

	data, err := json.Marshal(keyData)
	if err != nil {
		return fmt.Errorf("marshal identity: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write identity file: %w", err)
	}

	return nil
}
