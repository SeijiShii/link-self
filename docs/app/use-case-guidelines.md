# ユースケース別 推奨実装ガイドライン

**ステータス:** Phase 2  
**参照:** [データ同期コンセプト](../spec/data-sync-concept.md)、[ネットワーク概念](../spec/network-concept.md)、[ライブラリとして使う](using-linkself-as-library.md)

---

## 対象ユースケース

LinkSelf は以下のようなユースケースに適している。本ドキュメントでは、それぞれに対する推奨実装方針を示す。

| ユースケース | 例 | 特徴 |
|---|---|---|
| **A. 小規模閉域ネットワーク** | 訪問看護チーム、社内業務アプリ、同人サークル運営 | 100名程度のメンバーが参加する閉じたネットワークでのデータ共有 |
| **B. 個人マルチデバイス同期** | 勉強メモアプリ、個人日記、読書記録 | サーバーレス・シームレスにデバイス間でデータを同期 |

---

## A. 小規模閉域ネットワーク（〜100名）

### A.1 想定シナリオ

- 組織やグループが 1 つのネットワークを形成し、メンバー間でデータを共有する
- メンバーにはロール（役割）があり、ロールに応じてデータの読み書き範囲が異なる
- メンバーの追加・削除は管理者が行う

### A.2 Suite 設計

```go
client.Start(ctx, linkself.Config{
    SuiteID: "jp.example.team-notes",    // 逆ドメインで一意に
    Roles: linkself.RoleDefs{
        "viewer":  {},
        "editor":  {Includes: []string{"viewer"}},
        "admin":   {Includes: []string{"editor"}},
    },
})
```

**推奨事項:**
- SuiteID は逆ドメイン形式で衝突を防ぐ
- ロール DAG は最初からすべてのロールを定義しなくてよい。必要に応じて追加する
- ロールは 1 メンバー 1 つ。兼務が必要なら DAG で複合ロールを定義する（例: `editor_accountant`）

### A.3 テーブル設計と権限

```go
// 共有テーブル: メンバー全員が読めて、editor 以上が書ける
client.MyDB().Exec(ctx, `CREATE TABLE IF NOT EXISTS articles (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    body TEXT,
    created_at TEXT NOT NULL
)`)
client.MyDB().SetPermissions(ctx, "articles", linkself.Permissions{
    Read:   "viewer",    // viewer 以上 → 全メンバーが読める
    Write:  "editor",    // editor 以上が書き込み可
    Delete: "owner",     // 書いた本人のみ削除可
})

// 管理テーブル: admin のみ
client.MyDB().Exec(ctx, `CREATE TABLE IF NOT EXISTS member_settings (
    did TEXT PRIMARY KEY,
    display_name TEXT,
    status TEXT
)`)
client.MyDB().SetPermissions(ctx, "member_settings", linkself.Permissions{
    Read:   "members",   // 全員が読める
    Write:  "admin",     // admin のみ変更可
    Delete: "admin",
})

// 個人メモ: 本人のみ（同期は自デバイス間のみ）
client.MyDB().Exec(ctx, `CREATE TABLE IF NOT EXISTS my_drafts (
    id TEXT PRIMARY KEY,
    content TEXT
)`)
client.MyDB().SetPermissions(ctx, "my_drafts", linkself.Permissions{
    Read:   "self",      // 自分のデバイス間のみ同期
    Write:  "self",
    Delete: "self",
})
```

**推奨事項:**
- `read` 権限が同期範囲を決定する。`self` に設定したテーブルはネットワークに配信されない
- 行レベルの制御が必要な場合は、アプリ側で WHERE 句を使う
- テーブルの用途ごとに権限を分離する。1 テーブルに複数の権限要件を混在させない

### A.4 ネットワーク管理

```go
// ネットワーク作成（最初のメンバーが admin になる）
instanceID, _ := client.Network().Create(ctx, "東京チーム")

// メンバー追加と ロール割り当て
client.Network().AddMember(ctx, newMemberDID)
client.Network().SetMemberRole(ctx, newMemberDID, "editor")

// メンバー一覧取得
members, _ := client.Network().Members(ctx)
```

**推奨事項:**
- 管理操作（メンバー追加・削除・ロール変更）は admin ロールに限定する
- ネットワーク参加のオンボーディング手順をアプリの UI で案内する（DID の QR コード共有など）

### A.5 100 名規模での留意点

| 項目 | 方針 |
|------|------|
| **同期トラフィック** | テーブルの `read` 権限をロールで絞り、全員に配信するデータを最小限にする |
| **オフライン復帰** | ChangeLog retention（デフォルト 30 日）を超えた場合は自動フルシンクで復帰する。長期オフラインが多い場合は retention を延長する |
| **競合** | Last-write-wins（タイムスタンプ）で解決される。同時編集が頻繁な場合はアプリ側で楽観ロックを実装する |
| **データ量** | 現設計は MB〜GB 規模を想定。大きなファイル（画像等）はメタデータのみ同期し、ファイル本体は別経路を検討する |
| **メンバー増減** | P2P のため中央認証がない。メンバー除外後もそのメンバーの手元に既同期データは残る。機密データの扱いは運用設計で対処する |

---

## B. 個人マルチデバイス同期（サーバーレス）

### B.1 想定シナリオ

- 1 人のユーザーが複数デバイス（PC、スマホ、タブレット）で同じアプリを使う
- サーバーを用意せずに、デバイス間でデータがシームレスに同期される
- アカウント登録やログインは不要

### B.2 Suite 設計

```go
client.Start(ctx, linkself.Config{
    SuiteID: "jp.example.study-notes",
    // Roles は不要（個人利用のため）
})
```

**推奨事項:**
- 個人利用でもネットワークは必要（最低 1 メンバーの個人データ空間として機能する）
- Roles 定義は省略可能。将来グループ共有に拡張する可能性があるなら最初から定義しておくとよい

### B.3 テーブル設計と権限

```go
// すべてのテーブルを self 権限で定義（自デバイス間のみ同期）
client.MyDB().Exec(ctx, `CREATE TABLE IF NOT EXISTS notes (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    body TEXT,
    tags TEXT,
    updated_at TEXT NOT NULL
)`)
client.MyDB().SetPermissions(ctx, "notes", linkself.Permissions{
    Read:   "self",
    Write:  "self",
    Delete: "self",
})
```

**推奨事項:**
- 個人利用では全テーブルを `self` に設定する。DeviceSync のみが動作し、ネットワークへの配信は行われない
- 将来の共有化に備えて PRIMARY KEY は UUID 等のグローバルに一意な値にしておく

### B.4 デバイス追加（ペアリング）

```go
// === 既存デバイス側 ===
token, _ := client.CreatePairingToken(ctx)
// token.Secret を QR コードで表示

// === 新デバイス側 ===
client.CompletePairing(ctx, scannedSecret)
// → ユーザー鍵が転送され、既存データの全同期が始まる
```

**推奨事項:**
- ペアリングはアプリ内 UI で完結させる。QR コード表示 → 読み取りの一連の流れを案内する
- トークンは時間制限付き。期限切れの場合は再生成を促す
- 全デバイスを同時に失うとユーザー鍵を喪失しデータ復元不可。重要なデータのエクスポート機能（Dump）を提供することを推奨する

### B.5 デバイス間同期のふるまい

| 状況 | 動作 |
|------|------|
| **同一 LAN 内** | mDNS で自動発見し即時同期 |
| **異なるネットワーク** | DHT で発見し接続後に同期 |
| **片方がオフライン** | オンライン復帰時に ChangeLog ベースの差分同期 |
| **長期オフライン** | ChangeLog retention 超過時は自動フルシンク |
| **同時編集** | Last-write-wins（タイムスタンプ）。個人利用では衝突は稀 |

### B.6 個人アプリ → グループ共有への拡張

個人アプリを後からグループ共有に拡張する場合:

```go
// 1. Config にロールを追加（アプリのバージョンアップとして）
client.Start(ctx, linkself.Config{
    SuiteID: "jp.example.study-notes",
    Roles: linkself.RoleDefs{
        "member": {},
        "admin":  {Includes: []string{"member"}},
    },
})

// 2. 共有したいテーブルの同期スコープを変更
client.MyDB().SetSyncScope(ctx, "notes", linkself.ScopeNetwork, networkID,
    linkself.WithIncludeExisting(true))  // 既存データも共有

// 3. 権限を再設定
client.MyDB().SetPermissions(ctx, "notes", linkself.Permissions{
    Read:   "member",
    Write:  "member",
    Delete: "owner",
})
```

**推奨事項:**
- 個人→共有の移行は**アプリのバージョンアップ**として意識的に行う。気軽なトグルにしない
- スキーマのマイグレーションと合わせて同期スコープ・権限を設計し直す
- `IncludeExisting(true)` で既存データを共有するか、`false` で新規データのみ共有するかはアプリの要件に応じて選択する

---

## 共通の実装方針

### プラットフォーム統合

LinkSelf は Go コアとして提供される。アプリからの呼び出し方法はプラットフォームに応じて選択する。

| 方式 | 利点 | 適するケース |
|------|------|-------------|
| **サブプロセス + JSON-RPC** | 実装が単純、Go 側の変更が少ない | Electron、デスクトップアプリ |
| **FFI（DLL / .so / .dylib）** | プロセス間通信のオーバーヘッドなし | パフォーマンス重視のアプリ |
| **gomobile（AAR / Framework）** | ネイティブバインディング | Android / iOS |

詳細は [LinkSelf をライブラリとして使う](using-linkself-as-library.md) を参照。

### データバックアップ

```go
// データのダンプ（エクスポート）
data, _ := client.MyDB().Dump(ctx)

// データのリストア（インポート）
client.MyDB().Restore(ctx, data)
```

- P2P のため中央バックアップは存在しない。アプリでエクスポート/インポート UI を提供する
- 特に個人利用の場合、全デバイス喪失に備えたバックアップ手段を案内する

### エラーハンドリング

- LinkSelf への SQL クエリは通常の `database/sql` と同様にエラーを返す
- 権限エラーは明示的なエラーコードで返される。アプリ UI で適切にユーザーに伝える
- 同期エラー（ネットワーク断等）はアプリに伝播しない。再接続時に自動リトライされる
