// Package linkself provides the public API for interacting with LinkSelf nodes.
// This package abstracts the internal implementation details and provides a stable
// interface for applications to use LinkSelf functionality.
//
// The main entry point is the Client interface, which can be created using NewClient().
// All operations are performed through this interface, ensuring that applications
// don't need to depend on internal packages.
//
// Package linkself は、LinkSelfノードと対話するための公開APIを提供します。
// このパッケージは、内部実装の詳細を抽象化し、アプリケーションがLinkSelfの機能を
// 使用するための安定したインターフェースを提供します。
//
// メインのエントリーポイントは、NewClient()で作成できるClientインターフェースです。
// すべての操作はこのインターフェースを通じて実行され、アプリケーションが
// internalパッケージに依存する必要がなくなります。
//
// Example usage:
//
//	client := linkself.NewClient()
//	config := linkself.Config{
//		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
//	}
//	info, err := client.Start(ctx, config)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.Stop(ctx)
//
//	client.SetOnMessage(func(peerDID string, payload []byte) {
//		fmt.Printf("Received: %s\n", string(payload))
//	})
//
//	err = client.SendMessage(ctx, peerDID, "Hello!")
package linkself

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SeijiShii/link-self/core/internal/role"
)

// RetentionMode selects how ChangeLog entries are retained.
type RetentionMode int

const (
	// TimeBasedRetention retains entries for a duration (default: 30 days).
	TimeBasedRetention RetentionMode = iota
	// CountBasedRetention retains the most recent N entries (default: 10000).
	CountBasedRetention
)

// ChangeLogRetention configures ChangeLog pruning policy.
// Zero value uses TimeBased with 30 days.
type ChangeLogRetention struct {
	Mode     RetentionMode
	Duration time.Duration // for TimeBased (default: 30 days)
	MaxCount int           // for CountBased (default: 10000)
}

// SyncScope controls how a table's data is replicated.
type SyncScope int

const (
	// ScopeDevice syncs data only across the user's own devices (default).
	ScopeDevice SyncScope = iota
	// ScopeNetwork syncs data to all network members.
	ScopeNetwork
)

// SyncScopeOption configures SetSyncScope behavior.
type SyncScopeOption func(*syncScopeConfig)

type syncScopeConfig struct {
	includeExisting bool
}

// WithIncludeExisting controls whether existing data is synced when changing scope.
// Default is false (only new writes are shared).
func WithIncludeExisting(include bool) SyncScopeOption {
	return func(c *syncScopeConfig) {
		c.includeExisting = include
	}
}

// Config holds configuration for creating a LinkSelf node.
//
// All fields are optional and have sensible defaults:
//   - IdentityPath: defaults to ~/.linkself/identity.json
//   - ListenAddrs: defaults to ["/ip4/127.0.0.1/tcp/0"]
//   - BootstrapPeers: defaults to empty (no bootstrap peers)
//
// Config は、LinkSelfノードを作成するための設定を保持します。
//
// すべてのフィールドはオプションで、適切なデフォルト値があります:
//   - IdentityPath: デフォルトは ~/.linkself/identity.json
//   - ListenAddrs: デフォルトは ["/ip4/127.0.0.1/tcp/0"]
//   - BootstrapPeers: デフォルトは空（ブートストラップピアなし）
type Config struct {
	// IdentityPath is the path to the identity file.
	// If empty, defaults to ~/.linkself/identity.json.
	// The identity file stores the node's private key and DID.
	//
	// IdentityPath は、IDファイルのパスです。
	// 空の場合は、デフォルトで ~/.linkself/identity.json が使用されます。
	// IDファイルには、ノードの秘密鍵とDIDが保存されます。
	IdentityPath string

	// ListenAddrs are the addresses to listen on in multiaddr format.
	// If empty, defaults to ["/ip4/127.0.0.1/tcp/0"].
	//
	// ListenAddrs は、リッスンするアドレスのリスト（multiaddr形式）です。
	// 空の場合は、デフォルトで ["/ip4/127.0.0.1/tcp/0"] が使用されます。
	//
	// Examples:
	//   - "/ip4/127.0.0.1/tcp/4001" (IPv4 TCP)
	//   - "/ip6/::/tcp/4001" (IPv6 TCP)
	//   - "/ip4/0.0.0.0/tcp/4001" (all interfaces)
	ListenAddrs []string

	// BootstrapPeers are the bootstrap peer addresses in multiaddr format.
	// These peers are used for initial peer discovery in the DHT.
	//
	// BootstrapPeers は、ブートストラップピアのアドレス（multiaddr形式）です。
	// これらのピアは、DHTでの初期ピア発見に使用されます。
	//
	// Example: "/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWExamplePeerID"
	BootstrapPeers []string

	// StorageBackend selects the storage implementation for all internal stores.
	// Use MemoryBackend() for ephemeral in-memory storage (default when nil).
	// Use SQLiteBackend(path) for persistent SQLite3 storage.
	StorageBackend StorageBackend

	// Roles defines the role hierarchy (DAG) for this suite.
	// Keys are role names, values define which roles they include.
	// If nil, an empty DAG is used (only "members" permission works).
	Roles role.RoleDefs

	// AdminRole is the role name required for network management operations
	// (add member, kick, set role). Defaults to "admin" if empty.
	AdminRole string

	// ChangeLogRetention configures ChangeLog pruning policy.
	// If nil, defaults to TimeBased with 30 days.
	ChangeLogRetention *ChangeLogRetention

	// SuiteID identifies the application suite (e.g. "com.example.notes").
	// When set, LinkSelf stores data at a path determined by dataroot:
	//   <root>/<encodedDID>/suites/<suiteID>/data.db
	// When empty, an in-memory database is used.
	SuiteID string
}

// NodeInfo contains information about a started node.
//
// NodeInfo は、起動したノードの情報を保持します。
type NodeInfo struct {
	// DID is the node's Decentralized Identifier (did:key:...).
	// This is the unique identifier for this node in the LinkSelf network.
	//
	// DID は、ノードの分散識別子（did:key:...）です。
	// これは、LinkSelfネットワーク内でのこのノードの一意の識別子です。
	DID string

	// ListenAddr is the node's listen address in multiaddr format.
	// This includes both the network address and the peer ID.
	//
	// ListenAddr は、ノードのリッスンアドレス（multiaddr形式）です。
	// これには、ネットワークアドレスとピアIDの両方が含まれます。
	//
	// Example: "/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWExamplePeerID"
	ListenAddr string
}

// MessageHandler is a function type called when a message is received.
// The peerDID parameter is the DID of the peer who sent the message.
// The payload parameter contains the message content as bytes.
//
// MessageHandler は、メッセージを受信したときに呼び出される関数型です。
// peerDID パラメータは、メッセージを送信したピアのDIDです。
// payload パラメータは、メッセージの内容をバイトとして含みます。
type MessageHandler func(peerDID string, payload []byte)

// Client is the public API for interacting with a LinkSelf node.
// This interface abstracts the internal implementation details and provides
// a stable interface that applications can depend on.
//
// All methods must be called in order:
// 1. Create a client with NewClient()
// 2. Start the node with Start()
// 3. Use other methods (GetMyDID, SendMessage, Connect, SetOnMessage)
// 4. Stop the node with Stop() when done
//
// The client is not safe for concurrent use. If you need to use it from
// multiple goroutines, you must synchronize access yourself.
//
// Client は、LinkSelfノードと対話するための公開APIです。
// このインターフェースは、内部実装の詳細を抽象化し、アプリケーションが
// 依存できる安定したインターフェースを提供します。
//
// すべてのメソッドは順番に呼び出す必要があります:
// 1. NewClient()でクライアントを作成
// 2. Start()でノードを起動
// 3. 他のメソッド（GetMyDID, SendMessage, Connect, SetOnMessage）を使用
// 4. 完了したらStop()でノードを停止
//
// クライアントは並行使用に対して安全ではありません。複数のgoroutineから
// 使用する必要がある場合は、アクセスを同期する必要があります。
type Client interface {
	// Start starts a LinkSelf node with the given configuration.
	// It loads or generates an identity, creates a libp2p host, and starts
	// the node's services.
	//
	// The context can be used to cancel the start operation, but once started,
	// the node will continue running until Stop() is called.
	//
	// Returns NodeInfo containing the node's DID and listen address on success.
	// Returns an error if the node cannot be started (e.g., invalid configuration,
	// port already in use, etc.).
	//
	// Start は、指定された設定でLinkSelfノードを起動します。
	// IDを読み込むか生成し、libp2pホストを作成し、ノードのサービスを開始します。
	//
	// コンテキストは起動操作をキャンセルするために使用できますが、一度起動すると、
	// Stop()が呼ばれるまでノードは実行を続けます。
	//
	// 成功時は、ノードのDIDとリッスンアドレスを含むNodeInfoを返します。
	// ノードを起動できない場合（例: 無効な設定、ポートが既に使用されているなど）は
	// エラーを返します。
	Start(ctx context.Context, config Config) (*NodeInfo, error)

	// Stop stops the node and releases all resources.
	// After calling Stop(), the client should not be used again.
	// It is safe to call Stop() multiple times or if the node is not started.
	//
	// The context can be used to cancel the stop operation, but typically
	// you should use context.Background() or context.TODO().
	//
	// Stop は、ノードを停止し、すべてのリソースを解放します。
	// Stop()を呼び出した後は、クライアントを再度使用すべきではありません。
	// Stop()を複数回呼び出すか、ノードが起動していない場合でも安全です。
	//
	// コンテキストは停止操作をキャンセルするために使用できますが、通常は
	// context.Background()またはcontext.TODO()を使用すべきです。
	Stop(ctx context.Context) error

	// GetMyDID returns the node's DID.
	// Returns an empty string if the node is not started.
	// The DID format is "did:key:..." and uniquely identifies this node.
	//
	// GetMyDID は、ノードのDIDを返します。
	// ノードが起動していない場合は空文字列を返します。
	// DID形式は "did:key:..." で、このノードを一意に識別します。
	GetMyDID() string

	// SendMessage sends a message to a peer identified by their DID.
	// For 1-to-1 messaging, this creates a 2-person group internally.
	//
	// The peer must be connected (via Connect()) before messages can be sent.
	// Messages may be queued if the peer is not immediately available.
	//
	// Returns an error if:
	//   - The node is not started
	//   - The peerDID is invalid
	//   - The message cannot be sent (e.g., peer not connected)
	//
	// SendMessage は、DIDで識別されるピアにメッセージを送信します。
	// 1対1のメッセージングの場合、内部的に2人のグループが作成されます。
	//
	// メッセージを送信する前に、ピアは（Connect()経由で）接続されている必要があります。
	// ピアがすぐに利用できない場合、メッセージはキューに入れられる可能性があります。
	//
	// 以下の場合にエラーを返します:
	//   - ノードが起動していない
	//   - peerDIDが無効
	//   - メッセージを送信できない（例: ピアが接続されていない）
	SendMessage(ctx context.Context, peerDID string, message string) error

	// Connect connects to a peer identified by their DID and authenticates.
	// This also flushes any queued messages for this peer.
	//
	// The peerDID must be a valid DID in the format "did:key:...".
	// The connection process involves:
	//   1. Discovering the peer's network address via DHT
	//   2. Establishing a network connection
	//   3. Authenticating using the peer's DID
	//
	// Returns an error if:
	//   - The node is not started
	//   - The peerDID is invalid
	//   - The peer cannot be found or connected
	//
	// Connect は、DIDで識別されるピアに接続し、認証します。
	// これにより、このピアのキューに入れられたメッセージもフラッシュされます。
	//
	// peerDIDは "did:key:..." 形式の有効なDIDである必要があります。
	// 接続プロセスには以下が含まれます:
	//   1. DHT経由でピアのネットワークアドレスを発見
	//   2. ネットワーク接続を確立
	//   3. ピアのDIDを使用して認証
	//
	// 以下の場合にエラーを返します:
	//   - ノードが起動していない
	//   - peerDIDが無効
	//   - ピアが見つからないか接続できない
	//
	// listenAddr が非空のときは DHT 検索をスキップし、そのアドレスに直接接続する（後方互換用。推奨は DHT のみで peerDID のみ渡す）。
	Connect(ctx context.Context, peerDID string, listenAddr string) error

	// SetOnMessage sets the message handler callback.
	// The handler is called whenever a message is received from any peer.
	//
	// The handler can be set before or after Start(), but messages will only
	// be received after the node is started.
	//
	// If SetOnMessage is called multiple times, only the last handler is used.
	// If called with nil, message handling is disabled.
	//
	// SetOnMessage は、メッセージハンドラコールバックを設定します。
	// ハンドラは、任意のピアからメッセージを受信するたびに呼び出されます。
	//
	// ハンドラはStart()の前または後に設定できますが、メッセージは
	// ノードが起動した後にのみ受信されます。
	//
	// SetOnMessageが複数回呼び出された場合、最後のハンドラのみが使用されます。
	// nilで呼び出された場合、メッセージ処理は無効になります。
	SetOnMessage(handler MessageHandler)

	// MyDB returns the unified data API (KV + SQL).
	// Returns nil if the node is not started.
	MyDB() MyDB

	// Network returns the NetworkAPI for network management.
	// Returns nil if the node is not started.
	Network() NetworkAPI
}

// MyDB is the unified public API for data access.
// It provides both KV operations (Put/Get/Delete/List) and SQL operations
// (Exec/Query/Migrate). All writes are automatically replicated.
// Sync scope is controlled per-table via permissions (Phase C).
type MyDB interface {
	// Put stores a record in the given table. Replicated to all devices.
	Put(ctx context.Context, table, recordID string, body []byte) error

	// Get retrieves a record by table and ID. Returns nil, nil if not found.
	Get(ctx context.Context, table, recordID string) (*Record, error)

	// Delete marks a record as deleted. Replicated to all devices.
	Delete(ctx context.Context, table, recordID string) error

	// List returns all records in a table.
	List(ctx context.Context, table string) ([]*Record, error)

	// Dump returns all records across all tables.
	Dump(ctx context.Context) ([]*Record, error)

	// Restore applies records using last-write-wins by timestamp.
	// Returns the number of records actually applied.
	Restore(ctx context.Context, records []*Record) (int, error)

	// Exec executes a SQL statement (INSERT, UPDATE, DELETE, CREATE TABLE, etc.).
	Exec(ctx context.Context, sql string, args ...any) (DBResult, error)

	// Query executes a SELECT and returns rows.
	Query(ctx context.Context, sql string, args ...any) (DBRows, error)

	// QueryRow executes a SELECT returning at most one row.
	QueryRow(ctx context.Context, sql string, args ...any) DBRow

	// Migrate runs schema migrations in order, skipping already-applied versions.
	Migrate(ctx context.Context, migrations []Migration) error

	// SetSyncScope changes the sync scope of a table.
	// ScopeDevice (default): sync only across user's own devices.
	// ScopeNetwork: sync to network members too.
	// Use WithIncludeExisting(true) to also sync existing data.
	SetSyncScope(ctx context.Context, table string, scope SyncScope, opts ...SyncScopeOption) error
}

// Note: SharedDB has been removed from the public API (Phase B).
// GroupShare remains as an internal sync mechanism used by MyDB's sync scope.

// channelConfig holds optional channel settings.
type channelConfig struct {
	retention time.Duration
}

// ChannelOption configures optional channel settings.
type ChannelOption func(*channelConfig)

// WithRetention sets the data retention period for the channel.
// Zero (default) means permanent storage (master data).
func WithRetention(d time.Duration) ChannelOption {
	return func(c *channelConfig) {
		c.retention = d
	}
}

// NetworkAPI manages networks (membership).
type NetworkAPI interface {
	// CreateGroup creates a new group with the given member DIDs.
	// The creator (this node's DID) is automatically included.
	CreateGroup(ctx context.Context, memberDIDs []string) (groupID string, err error)

	// AddMember adds a member to a group.
	AddMember(ctx context.Context, groupID, memberDID string) error

	// Leave removes this node from the group.
	Leave(ctx context.Context, groupID string) error

	// ListGroups returns all group IDs this node belongs to.
	ListGroups(ctx context.Context) ([]string, error)
}

// Note: DB interface has been removed from the public API (Phase B).
// SQL operations are now part of MyDB.

// DBResult is the result of an Exec operation.
type DBResult interface {
	LastInsertId() (int64, error)
	RowsAffected() (int64, error)
}

// DBRows is the result of a Query operation.
type DBRows interface {
	Columns() ([]string, error)
	Next() bool
	Scan(dest ...any) error
	Close() error
}

// DBRow is the result of a QueryRow operation.
type DBRow interface {
	Scan(dest ...any) error
}

// Migration defines a schema migration step.
type Migration struct {
	Version int
	SQL     string
}

// Record represents a stored item in DeviceDB.
type Record struct {
	ID        string          `json:"id"`
	Table     string          `json:"table"`
	Body      json.RawMessage `json:"body"`
	Timestamp int64           `json:"timestamp"`
}

// SharedRecord represents a shared item in GroupShare.
type SharedRecord struct {
	ID        string          `json:"id"`
	Channel   string          `json:"channel"`
	Topic     string          `json:"topic,omitempty"`
	GroupID   string          `json:"groupID"`
	DID       string          `json:"did"`
	Body      json.RawMessage `json:"body"`
	Timestamp int64           `json:"timestamp"`
}
