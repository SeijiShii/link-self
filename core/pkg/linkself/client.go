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
	"github.com/SeijiShii/link-self/core/internal/groupshare"
	"github.com/SeijiShii/link-self/core/internal/network"
	"github.com/SeijiShii/link-self/core/internal/role"
	"github.com/SeijiShii/link-self/core/internal/node"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// client implements the Client interface using internal packages.
// This is the only place where internal packages are accessed.
// All other code should use the Client interface instead of this concrete type.
type client struct {
	node           *node.Node
	identity       *did.Identity
	myDB           *myDB
	sharedDB       *sharedDB
	network        *networkAPI
	storageBackend StorageBackend
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

	// Resolve storage backend: nil defaults to MemoryBackend.
	backend := config.StorageBackend
	if backend == nil {
		backend = MemoryBackend()
	}
	// If it's a SQLite backend, open the database now.
	if sb, ok := backend.(*sqliteBackend); ok {
		if err := sb.open(); err != nil {
			n.Close()
			return nil, fmt.Errorf("open sqlite backend: %w", err)
		}
	}
	c.storageBackend = backend
	stores := backend.storages()

	// Wire DeviceSync layer (no peers initially — single device).
	dsStorage := stores.deviceStorage
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
	c.myDB = &myDB{engine: dsEngine}

	// Wire Network layer with role DAG.
	netStore := stores.networkStore
	var dag *role.DAG
	if config.Roles != nil {
		var err error
		dag, err = role.NewDAG(config.Roles)
		if err != nil {
			n.Close()
			return nil, fmt.Errorf("build role DAG: %w", err)
		}
	} else {
		dag, _ = role.NewDAG(role.RoleDefs{})
	}
	adminRole := config.AdminRole
	if adminRole == "" {
		adminRole = "admin"
	}
	netService := network.NewService(netStore, dag, adminRole)
	c.network = &networkAPI{
		service: netService,
		store:   netStore,
		selfDID: identity.DID,
	}

	// Wire GroupShare layer.
	gsStorage := stores.sharedStorage
	gsResolver := &memberResolverAdapter{store: netStore, selfDID: identity.DID}
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
	gsLayer.RemoteSubs = stores.subscriptionStore
	gsLayer.SendSubAnnounce = func(ctx context.Context, memberDIDs []string, payload []byte) error {
		wrapped, err := envelope.Wrap(envelope.TypeSubAnnounce, payload)
		if err != nil {
			return err
		}
		return n.SendToGroup(ctx, memberDIDs, wrapped)
	}
	c.sharedDB = &sharedDB{layer: gsLayer}

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
	var firstErr error
	if c.node != nil {
		if err := c.node.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		c.node = nil
		c.identity = nil
	}
	if c.storageBackend != nil {
		if err := c.storageBackend.close(); err != nil && firstErr == nil {
			firstErr = err
		}
		c.storageBackend = nil
	}
	return firstErr
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

// MyDB returns the MyDB interface. Returns nil before Start.
func (c *client) MyDB() MyDB {
	if c.myDB == nil {
		return nil
	}
	return c.myDB
}

// SharedDB returns the SharedDB interface. Returns nil before Start.
func (c *client) SharedDB() SharedDB {
	if c.sharedDB == nil {
		return nil
	}
	return c.sharedDB
}

// Network returns the NetworkAPI interface. Returns nil before Start.
func (c *client) Network() NetworkAPI {
	if c.network == nil {
		return nil
	}
	return c.network
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
