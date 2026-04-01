# LinkSelf 仕様概要

本ドキュメントは LinkSelf の現在の仕様と実装状態をまとめたものである。

---

## アーキテクチャ

LinkSelf はアプリから見た**ストレージプロキシ**として機能する。アプリは SQL クエリを発行するだけで、デバイス間・メンバー間の同期は LinkSelf が透過的に処理する。

```
アプリ → client.MyDB() → SQL / KV → 自動同期（DeviceSync / GroupShare）
```

### 公開 API

| API | 説明 |
|-----|------|
| `client.MyDB()` | 唯一のデータ API。SQL（Exec/Query/Migrate）+ KV（Put/Get/List）+ 同期スコープ設定 |
| `client.Network()` | ネットワーク管理（メンバー追加・脱退・ロール割り当て） |
| `client.CreatePairingToken()` / `CompletePairing()` | デバイスペアリング |
| `client.SendMessage()` / `Connect()` | 1対1メッセージング |

### 内部メカニズム（アプリからは非公開）

| パッケージ | 説明 |
|-----------|------|
| `devicesync` | 同一ユーザーの全デバイス間の透過同期（Last-write-wins） |
| `groupshare` | ネットワークメンバー間の同期（権限制御付き） |
| `sqlproxy` | SQL → ローカル SQLite 実行 → 書き込み検知 → 同期トリガー |

---

## 主要概念

| 概念 | 説明 |
|------|------|
| **Suite** | アプリ群の識別子（SuiteID）。開発者がビルド時にハードコード。スキーマ・ロール定義を共有する |
| **Network** | スイート内のメンバー集合 + データ空間。ユーザーが実行時に作成。最低 1 人（個人用） |
| **ユーザー DID** | ネットワークに公開される安定した DID。デバイスが変わっても不変 |
| **デバイス DID** | 各デバイス固有の DID。ペアリングにのみ使用。ネットワークからは見えない |
| **ロール DAG** | アプリが定義するロール階層。テーブル権限と管理操作の権限を制御 |
| **同期スコープ** | テーブル単位の設定。ScopeDevice（自デバイスのみ）/ ScopeNetwork（メンバー全員） |

---

## Config

```go
type Config struct {
    IdentityPath       string                // DID 秘密鍵のパス
    SuiteID            string                // アプリスイート識別子
    Roles              role.RoleDefs          // ロール DAG 定義
    AdminRole          string                // 管理操作に必要なロール（デフォルト: "admin"）
    ListenAddrs        []string              // リッスンアドレス
    BootstrapPeers     []string              // DHT ブートストラップピア
    ChangeLogRetention *ChangeLogRetention   // ChangeLog 保持ポリシー
}
```

SuiteID 指定時、ストレージは `<dataroot>/<encodedDID>/suites/<suiteID>/data.db` に自動配置される。

---

## 仕様書一覧

| ドキュメント | 内容 |
|------------|------|
| [data-sync-concept.md](data-sync-concept.md) | **メイン仕様**。ストレージプロキシモデル、Suite/Network、権限モデル、SQL インターフェース |
| [data-sync-decisions.md](data-sync-decisions.md) | 設計決定記録。検討事項、仕様精査結果 |
| [sync-db-plan.md](sync-db-plan.md) | DeviceSync / GroupShare 二層アーキテクチャの詳細設計 |
| [network-concept.md](network-concept.md) | ネットワーク（旧グループ）の概念、ロール DAG |
| [dump-restore-retention.md](dump-restore-retention.md) | Dump/Restore、Retention、ChangeLog 保持ポリシー |
| [topic-subscription-filtering.md](topic-subscription-filtering.md) | トピックベースのサブスクリプションフィルタリング |
| [phase1-design.md](phase1-design.md) | Phase 1 コアロジック設計（DID, DHT, Auth, Store-and-Forward） |
| [linkself-data-persistence-plan.md](linkself-data-persistence-plan.md) | データ永続化方針、ディレクトリ構造 |
| [group-syncdb-implementation.md](group-syncdb-implementation.md) | 旧グループ・同期 DB の実装記録（歴史的参照） |

---

## 実装済み機能

- MyDB 統合 API（SQL + KV + 同期スコープ）
- DeviceSync / GroupShare 二層同期
- ロール DAG + ネットワーク管理
- テーブル単位の権限（read/write/delete）
- SyncWith ハンドシェイク + 全同期フォールバック
- ChangeLog 保持ポリシー（時間/件数ベース）
- ユーザー鍵 / デバイス鍵の2層構造 + ペアリングプロトコル
- Config.SuiteID + dataroot ストレージ自動配置
- SQLite3 ストレージバックエンド（WAL モード）
- SubAnnouncement 再接続ハンドシェイク
- MyDB Dump/Restore
- JSON-RPC daemon
