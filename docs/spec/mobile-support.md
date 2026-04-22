# LinkSelf モバイル対応仕様

**日本語**（このページ）  
**ステータス:** 設計確定・未実装（M1 成果物）
**参照:**
- [LinkSelf 仕様概要](overview.md)
- [Phase 1 設計](phase1-design.md)
- [ネットワークの概念](network-concept.md)
- 原案: [chat-client/docs/LinkSelf拡張要望.md](../../chat-client/docs/LinkSelf拡張要望.md)

---

## 1. 背景

Home Visit Suite（管理・編集用デスクトップアプリ + 活動メンバー用モバイルアプリの 2 系統）のうちモバイル側を iOS/Android ネイティブで構築するため、LinkSelf に以下が必要になる:

- **Expo (React Native) + gomobile** で `pkg/linkself` を iOS/Android 双方に組み込み可能にする
- モバイル OS のバックグラウンド制約（iOS ~30s suspend、Android Doze Mode）と整合するライフサイクル API
- 前景起動時の短時間で同期完了させるための高速起動機構

本仕様は chat-client 側で起草された原案を LinkSelf 正式仕様として確定したもの。原案は削除または本仕様への参照リンクに縮退させる予定。

## 2. 前提

### 2.1 モバイルアプリの動作モデル

- 活動メンバーは訪問活動中に**前景でのみ**アプリを使う
- バックグラウンドでの常時同期は**不要**（次回起動時の差分同期で十分）
- iOS Background Mode（`voip` / `audio` / `background-fetch` 等）は**宣言しない**
- ライフサイクル想定: 前景起動 → 高速同期 → 背面遷移で graceful stop

### 2.2 技術前提

- Go モジュール: `github.com/SeijiShii/link-self/core`（Go 1.26 以降）
- libp2p: `go-libp2p v0.47.x`、`go-libp2p-kad-dht v0.37.x`
- モバイルビルド: `golang.org/x/mobile/cmd/gomobile bind`

### 2.3 gomobile bind の型制約

`gomobile bind` は以下を export 不可:

- `context.Context`
- func 型引数（`MessageHandler` 等）
- `...any` / `interface{}`（`Exec` / `Query` の可変長引数）
- `map[K]V`（`Config.Roles`）
- 複合型の型エイリアス（`json.RawMessage` 等、環境依存）

本仕様ではこれらを全て gomobile-safe な形で包むファサード層を追加する。

---

## 3. 設計方針（確定事項）

設計判断は以下の通り確定している:

| # | 項目 | 決定 | 根拠 |
|---|-----|------|------|
| 1 | `Pause` / `Resume` の配置 | `pkg/linkself.Client` interface に追加 | 将来デスクトップでも suspend/resume が使える余地を残す。現状 `Client` 実装者は daemon 内部のみで破壊リスクなし |
| 2 | Routing table snapshot | **実装しない**。peerstore 永続化のみ | RT は Resume 時に peerstore から再充填。公式 snapshot API がなく投資対効果が低い |
| 3 | peerstore バックエンド | `go-ds-leveldb`（pure Go） | CGO 不要で iOS arm64 simulator でのトラブル回避 |
| 4 | `mobile` の collection 戻り値 | 全て JSON 文字列に統一 | gomobile の型サポートの揺れを回避。ABI 互換を守る |
| 5 | context 扱い | `timeoutMs int64` 統一（0=default、正値=ms、負値=無制限） | gomobile-safe かつ最小 API |
| 6 | gomobile CI 実行頻度 | iOS bind は main push のみ、Android bind は PR ごと | macOS runner 料金と回帰検知のバランス |
| 7 | Listener 登録 | 単一 Listener（上書き方式） | 複数購読はネイティブ側で fan-out するのが一般的。最小 API に留める |
| 8 | `mobile.StartConfig` 設計 | 独立型 + 内部変換関数 | gomobile-safe に限定。埋め込みは型制約で詰む |
| 9 | iOS Background Mode | 宣言なし（前景のみ） | App Store 審査リスク回避。提案書前提と一致 |
| 10 | `SyncPreferTables` | ヒント保存のみ（並べ替え only） | 提案書が明示的に最低限でよいとしている。実機計測後に深掘り検討 |

---

## 4. スコープ

### 4.1 必須（本仕様でカバー）

- **4.1.1 gomobile ファサード API**: `pkg/linkself/mobile` サブパッケージ新設
- **4.1.2 Foreground/Background ライフサイクル API**: `Client.Pause` / `Client.Resume`
- **4.1.3 高速起動モード**: `Config.FastStart`、`Config.KnownPeerHints`
- **4.1.4 peerstore 永続化**: `Config.PeerstorePath` + 定期フラッシュ

### 4.2 推奨（本仕様でカバー）

- **4.2.1 Circuit Relay v2 クライアント対応**: `Config.CircuitRelays`
- **4.2.2 差分同期の優先度制御**: `MyDB.SyncPreferTables` / `mobile.MobileDB.SyncPreferTables`
- **4.2.3 gomobile CI**: GitHub Actions で bind ビルド検証

### 4.3 本仕様のスコープ外

- APNs / FCM 連携（将来検討）
- Android Doze Mode / iOS Low Power Mode の省電力同期（将来検討）
- 優先キュー化された差分同期（4.2.2 はヒント only）

---

## 5. API 設計

### 5.1 `pkg/linkself.Client` への追加

```go
// types.go
type Client interface {
    // ...既存メソッド...

    // Pause は graceful に active 接続を閉じ、peerstore をディスクに flush し、
    // 背景同期タスクを停止可能な状態にする。モバイルの背面遷移時に呼ぶ。
    // ノードは論理的には "started" のまま残り、Resume で再開できる。
    Pause(ctx context.Context) error

    // Resume は永続化された peerstore から既知ピアへ再接続し、同期タスクを
    // 再開する。前景復帰時に呼ぶ。FastStart=false の場合は DHT bootstrap も
    // 再実行する。
    Resume(ctx context.Context) error
}
```

### 5.2 `pkg/linkself.Config` への追加

```go
type Config struct {
    // ...既存フィールド...

    // FastStart は Start() 時の DHT full bootstrap をスキップする。
    // PeerstorePath（前回保存分）または KnownPeerHints のいずれかに
    // 既知ピア情報があることが前提。
    FastStart bool

    // KnownPeerHints は Start 前に peerstore に注入する既知ピアの
    // multiaddr リスト。モバイルアプリがビルド時やユーザー設定で
    // home-node のアドレスを知っている場合に使う。
    KnownPeerHints []string

    // PeerstorePath は peerstore（および将来の routing table snapshot）
    // を永続化するディレクトリ。空文字の場合は
    // "<dataroot>/<encodedDID>/peerstore" を使う。
    PeerstorePath string

    // CircuitRelays は libp2p Circuit Relay v2 のリレーノード。
    // BootstrapPeers とは別枠で、リレー用途であることを明示する。
    // CGNAT 下のモバイルを想定。
    CircuitRelays []string
}
```

### 5.3 `pkg/linkself.MyDB` への追加

```go
type MyDB interface {
    // ...既存メソッド...

    // SyncPreferTables は次回同期バッチで優先するテーブルを宣言する。
    // ハードキューではなく best-effort hint。複数回呼ばれた場合は
    // 最後の指定で上書きされる。空スライスでクリア。
    SyncPreferTables(ctx context.Context, tables []string) error
}
```

### 5.4 `pkg/linkself/mobile` サブパッケージ

gomobile bind 対象。`linkself.Client` / `linkself.MyDB` / `linkself.NetworkAPI` を wrap する。

#### 5.4.1 `StartConfig`

```go
// core/pkg/linkself/mobile/types.go
package mobile

// StartConfig は Config の gomobile-safe 版。
// map / interface / func 型を含まず、全フィールドが primitive または
// JSON 文字列のみで構成される。
type StartConfig struct {
    IdentityPath       string
    SuiteID            string
    AdminRole          string
    ListenAddrsJSON    string // JSON: ["/ip4/0.0.0.0/tcp/0", ...]
    BootstrapPeersJSON string // JSON 配列
    CircuitRelaysJSON  string // JSON 配列
    KnownPeerHintsJSON string // JSON 配列
    RolesJSON          string // JSON: {"admin":{"includes":["nurse"]}, ...}
    DataRootDir        string // NSDocumentDirectory / filesDir をネイティブ側から注入
    PeerstorePath      string // 空なら <DataRootDir>/<encodedDID>/peerstore
    StartTimeoutMs     int64  // 内部で context.WithTimeout に変換
    FastStart          bool
}

type StartResult struct {
    DID        string
    ListenAddr string
}
```

#### 5.4.2 Listener インタフェース

```go
// core/pkg/linkself/mobile/listeners.go

// MessageListener は gomobile bind 経由で Obj-C/Java 側が実装可能。
// MobileClient は単一 Listener のみ保持し、SetMessageListener(nil) で解除。
type MessageListener interface {
    OnMessage(peerDID string, payload []byte)
}

type LifecycleListener interface {
    OnPaused()
    OnResumed()
    OnSyncProgress(table string, done int64, total int64)
}
```

#### 5.4.3 `MobileClient`

```go
// core/pkg/linkself/mobile/client.go

type MobileClient struct { /* 非公開フィールド */ }

func NewMobileClient() *MobileClient

// ライフサイクル
func (c *MobileClient) Start(cfg *StartConfig) (*StartResult, error)
func (c *MobileClient) Stop(timeoutMs int64) error
func (c *MobileClient) Pause(timeoutMs int64) error
func (c *MobileClient) Resume(timeoutMs int64) error

// 基本操作
func (c *MobileClient) GetMyDID() string
func (c *MobileClient) SendMessage(peerDID, message string, timeoutMs int64) error
func (c *MobileClient) Connect(peerDID, listenAddr string, timeoutMs int64) error

// Listener（単一登録・nil で解除）
func (c *MobileClient) SetMessageListener(l MessageListener)
func (c *MobileClient) SetLifecycleListener(l LifecycleListener)

// サブ API
func (c *MobileClient) DB() *MobileDB
func (c *MobileClient) Net() *MobileNetwork

// テスト用（pkg/linkself.Client の GenerateTestDID / InjectTestMessage を wrap）
func (c *MobileClient) GenerateTestDID() (string, error)
func (c *MobileClient) InjectTestMessage(fromDID string, payload []byte, timeoutMs int64) error
```

#### 5.4.4 `MobileDB`

```go
// core/pkg/linkself/mobile/db.go

type MobileDB struct { /* 非公開フィールド */ }

// KV
func (d *MobileDB) Put(table, recordID string, body []byte, timeoutMs int64) error
func (d *MobileDB) Get(table, recordID string, timeoutMs int64) (*MobileRecord, error)
func (d *MobileDB) Delete(table, recordID string, timeoutMs int64) error
// 戻り値は []*MobileRecord を JSON 化（Q4: collection は全 JSON 統一）
func (d *MobileDB) ListJSON(table string, timeoutMs int64) (string, error)
func (d *MobileDB) DumpJSON(timeoutMs int64) (string, error)

// SQL（args は JSON 配列で渡す: 例 `["alice", 42, null]`）
func (d *MobileDB) Exec(sql, argsJSON string, timeoutMs int64) (*ExecResult, error)
func (d *MobileDB) QueryJSON(sql, argsJSON string, timeoutMs int64) (string, error)
func (d *MobileDB) QueryRowJSON(sql, argsJSON string, timeoutMs int64) (string, error)

// Migrate は {"version":1,"sql":"..."} のリストを JSON で受ける
func (d *MobileDB) Migrate(migrationsJSON string, timeoutMs int64) error

// 同期スコープ（0=Device, 1=Network）
func (d *MobileDB) SetSyncScope(table string, scope int, includeExisting bool, timeoutMs int64) error

// 優先同期ヒント（4.2.2）
func (d *MobileDB) SyncPreferTables(tablesJSON string) error

type ExecResult struct {
    LastInsertID int64
    RowsAffected int64
}

// json.RawMessage を []byte に置換した gomobile-safe 版
type MobileRecord struct {
    ID        string
    Table     string
    Body      []byte
    Timestamp int64
}
```

`QueryJSON` の戻り値 JSON 形式:

```json
{
  "columns": ["id", "name", "age"],
  "rows": [
    ["1", "alice", 30],
    ["2", "bob", null]
  ]
}
```

#### 5.4.5 `MobileNetwork`

```go
// core/pkg/linkself/mobile/network.go

type MobileNetwork struct { /* 非公開フィールド */ }

func (n *MobileNetwork) CreateGroup(memberDIDsJSON string, timeoutMs int64) (string, error)
func (n *MobileNetwork) AddMember(groupID, memberDID string, timeoutMs int64) error
func (n *MobileNetwork) Leave(groupID string, timeoutMs int64) error
func (n *MobileNetwork) ListGroupsJSON(timeoutMs int64) (string, error)
func (n *MobileNetwork) GetGroupJSON(groupID string, timeoutMs int64) (string, error)
```

---

## 6. 内部実装方針（`core/internal/*`）

提案書原案は「`pkg/linkself/` のみ変更、internal 不変」としているが、これは 4.1.1（ファサード）にのみ妥当。以下の項目は `internal/node` への追加が不可避:

- **4.1.2 Pause/Resume**: Host の active 接続列挙 + graceful close、peerstore flush
- **4.1.3 FastStart**: `internal/node/node.go` の DHT Bootstrap 呼び出しを条件分岐
- **4.1.4 peerstore 永続化**: `libp2p.New()` に `libp2p.Peerstore()` option を渡す必要
- **4.2.1 Circuit Relay v2**: `libp2p.EnableAutoRelayWithStaticRelays` option を適用

ただし**既存 API は破壊しない**。`pkg/linkself.Config` へのフィールド追加と `Client` interface への Pause/Resume 追加のみ。

### 6.1 新規ファイル（予定）

- `core/internal/node/peerstore_persist.go`: `go-ds-leveldb` ベースの永続 peerstore
- `core/internal/node/lifecycle.go`: `Pause()` / `Resume()` 実装
- `core/internal/node/relay.go`: Circuit Relay v2 クライアント設定
- `core/internal/devicesync/priority.go`: `SyncPreferTables` のヒント保持

### 6.2 peerstore 永続化の仕様

- 保存先: `<DataRoot>/<encodedDID>/peerstore/`（leveldb ディレクトリ）
- 書き込みタイミング:
  - `Pause()` 呼び出し時に明示 flush
  - `Stop()` 時に flush
  - 通常動作中も libp2p が自動的に leveldb に書き込む（pstoreds の標準動作）
- 復元タイミング: `Start()` 時に leveldb を open するだけで復元完了

### 6.3 Pause / Resume の動作

**Pause(ctx)**:
1. LifecycleListener.OnPaused() を呼ぶ（ネイティブ側に停止開始通知）
2. 同期タスクを中断（devicesync / groupshare のワーカーに stop シグナル）
3. Host の active 接続を graceful close（`host.Network().ClosePeer(id)`）
4. peerstore を明示 flush（leveldb WriteSync）
5. DHT を停止

iOS ~30s 制約に対し、これらは best-effort に 3 秒以内で完了する設計。間に合わない処理は打ち切り、次回 Resume で整合を取る。

**Resume(ctx)**:
1. peerstore を再 open（既に開いている場合はスキップ）
2. DHT を再起動
3. FastStart=true → DHT bootstrap スキップ、KnownPeerHints と peerstore の既知ピアに直接 dial
4. FastStart=false → DHT full bootstrap 実施
5. 同期タスクを再開
6. LifecycleListener.OnResumed() を呼ぶ

### 6.4 daemon (`core/cmd/linkself-daemon`) への波及

`Client` interface に `Pause` / `Resume` が増えるが、daemon 側で JSON-RPC として露出するかは任意。モバイルは gomobile 直結で使うため、本仕様では daemon への追加は**しない**。将来デスクトップが必要とした時に別 PR で追加する。

---

## 7. CI

### 7.1 新規 workflow

- `.github/workflows/ci.yml`: Go test（既存相当、未整備なら新設）
- `.github/workflows/gomobile.yml`: gomobile bind 検証

### 7.2 gomobile bind ジョブ

| ジョブ | Runner | トリガー |
|-------|--------|---------|
| Android bind | ubuntu-latest | PR + main push |
| iOS bind | macos-14 | main push のみ（PR では skip） |

対象パッケージ: `./core/pkg/linkself/mobile`

失敗時は即 regression 扱い。main 破損時は revert または即 fix PR で対応する。

---

## 8. テスト戦略

| レベル | 手段 | カバー範囲 |
|-------|------|----------|
| 単体 | `go test ./core/pkg/linkself/mobile/...` | ファサードの型変換、JSON 変換、Listener 配線 |
| 単体 | `go test ./core/internal/node/...` | peerstore persist/restore ラウンドトリップ、Pause/Resume 正常系・異常系 |
| 統合 | `core/test/integration/` | A が Pause → B がメッセージ送信 → A が Resume → 受信、2 ノードのウォームスタート時間 |
| bind ビルド | CI (`gomobile bind`) | iOS/Android 両方のビルド成功を継続的に検証 |
| 実機 PoC | 手動（M5 マイルストーン） | cold start → DID 取得・差分同期完了までの計測 |

自動化できない計測項目は `docs/spec/mobile-measurements.md`（M5 で新設）に記録する。

---

## 9. マイルストーン

| フェーズ | 内容 | 成果物 |
|---------|------|--------|
| **M1** | 机上調査・設計確定 | 本ドキュメント |
| **M2** | gomobile bind 試験（現状 `pkg/linkself`） | ビルドログ、失敗点リスト |
| **M3** | モバイルファサード実装（§5.4） | `pkg/linkself/mobile` 初版、シミュレータで最小動作 |
| **M4** | ライフサイクル API 実装（§5.1 / §5.2 / §6） | Pause/Resume、FastStart、peerstore 永続化 |
| **M5** | 実機 PoC + 計測 | 起動時間・同期時間の実測値、`mobile-measurements.md` |
| **M6** | Circuit Relay v2 + SyncPreferTables + CI | リレー対応、優先同期ヒント、CI green |

### 9.1 依存関係

- M3 は M2 の結果（実 bind で詰まる型の最終リスト）を受ける
- M4 は M3 のファサードから Pause/Resume を呼ぶため M3 先行
- M5 は M3/M4 完了後の実機検証
- M6 は M5 の実機計測で Relay 必要性が確認されてから着手

---

## 10. 既知の制約・注意事項

- **iOS Background Mode 未宣言**: 背面遷移後は完全 suspend 想定。バックグラウンド通知は将来 APNs 経由で別途実装
- **Routing table 永続化なし**: Resume 直後の数秒は DHT lookup が遅い可能性。UX に影響が出たら routing table snapshot を追加検討
- **`SyncPreferTables` は best-effort**: 優先キュー化された厳密な順序保証ではない
- **`libp2p.EnableAutoRelayWithStaticRelays` の deprecation**: go-libp2p issue で将来的な API 変更が予告されている。M6 実装時に最新状況を確認すること
- **quic-go / pion-webrtc の cgo 事情**: iOS simulator ビルドで突発的に build が落ちる可能性があり、CI で早期検知を行う

---

## 11. 参考

- [Berty](https://berty.tech/) — go-libp2p を iOS/Android で App Store/Play Store 配信している先例
- [Briar](https://briarproject.org/) — iOS 版を断念している事例（常時バックグラウンド通信が前提のため）
- [gomobile](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile) — 型制約の公式ドキュメント
- 原案: [chat-client/docs/LinkSelf拡張要望.md](../../chat-client/docs/LinkSelf拡張要望.md)
