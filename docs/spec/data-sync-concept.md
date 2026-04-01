# データ同期コンセプト

**ステータス:** Phase 1（概念整理）
**参照:** [sync-db-plan.md](sync-db-plan.md)、[linkself-data-persistence-plan.md](linkself-data-persistence-plan.md)、[network-concept.md](network-concept.md)
**設計決定記録:** [data-sync-decisions.md](data-sync-decisions.md)

---

## 1. 前提

ここで扱うテーマは LinkSelf ネットワーク内でのデータ同期についてである。

LinkSelf はアプリから見た**ストレージそのもの**として機能する。

```
アプリ → LinkSelf（SQL クエリ受付・データ返却・同期は透過的）
```

**LinkSelf が提供するもの:**
- SQLite と同等のクエリインターフェース（WHERE, JOIN, INDEX）
- データ同期の完全な透過性（アプリは同期を意識しない）
- ネットワーク内の権限管理

**アプリが意識するのは:**
- ネットワーク内での権限（誰がどのデータを読み書きできるか）

**アプリが意識しなくてよいもの:**
- データがどのデバイスにあるか
- 同期のタイミング・方法・競合解決
- ストレージの物理的な配置
- データストアの実装（SQLite 等）

LinkSelf はアプリとストレージの間に立つ**プロキシ**として働く。アプリから見れば SQLite に直接アクセスしているのと変わらないが、裏側でデバイス間・メンバー間の同期が自動的に行われる。

### データストアの完全内包

データストア実装（SQLite3）は LinkSelf が完全に内包する。アプリがストレージ実装を提供したり、内部のストレージインターフェースにアクセスしたりすることはない。

- LinkSelf が SQLite3 のライフサイクル（作成・マイグレーション・バックアップ）を管理する
- アプリは SQL クエリを発行するだけで、その先の永続化・同期・競合解決はすべて LinkSelf の責務
- ストレージのパスは LinkSelf が DID 空間・SuiteID・ネットワークインスタンスに基づいて自動決定する。アプリはストレージの配置にも実装にも関与しない

---

## 2. 用語の定義

| 概念 | 用語 | 定義 |
|------|------|------|
| アプリ群の識別 | **スイート (Suite)** | 同じスキーマ・ロール定義を共有するアプリ群。SuiteID で識別。開発者がハードコードする |
| メンバーの集合 + データ空間 | **ネットワーク (Network)** | スイート内の具体的なメンバー集合とデータ空間。ユーザーが実行時に作成する |
| ネットワークに属する DID | **メンバー (Member)** | ネットワークの参加者。「ユーザー」は LinkSelf 外の文脈で使い、データモデル内では「メンバー」に統一する |
| アプリ向け SQL インターフェース | **MyDB** | アプリが使う唯一の公開 API。SQL クエリを受け付け、テーブルの同期スコープ設定に基づいて自動同期する |
| 同一 DID 間の透過同期 | **DeviceSync** | 内部メカニズム。同一ユーザーの全デバイスへの透過同期を担う |
| メンバー間の共有同期 | **SharedDB** | 内部メカニズム。ネットワークメンバー間のデータ同期と権限制御を担う |
| ユーザーの識別子 | **ユーザー DID** | ネットワークに公開される安定した DID。デバイスが変わっても不変 |
| デバイスの識別子 | **デバイス DID** | 各デバイス固有の DID。ペアリングにのみ使用し、ネットワークには公開されない |

### スイートとネットワーク

この2つは異なるレイヤーの概念である。

| | スイート (Suite) | ネットワーク (Network) |
|---|---|---|
| **誰が決める** | アプリ開発者 | ユーザー |
| **いつ決まる** | ビルド時（ハードコード） | 実行時（作成・参加） |
| **識別子** | SuiteID | ネットワークインスタンス ID |
| **何を定義する** | スキーマ、ロール定義、テーブル権限 | メンバー集合、ロール割り当て、データ |
| **例** | 訪問看護スイート | 東京クリニック、大阪クリニック |

1つの DID は同じスイートの複数のネットワークに所属できる（同じアプリの複数インスタンス）。

```
訪問看護スイート（SuiteID: "home-visit-suite"）
  ├── 東京クリニックのネットワーク（田中、佐藤、山田）
  └── 大阪クリニックのネットワーク（鈴木、高橋、田中）
      ↑ 田中は両方に所属
```

---

## 3. スイート、ネットワーク、アプリの関係

### SuiteID

スイートの識別子。アプリ開発者がハードコードする。

- 同じ SuiteID を持つ異なるアプリ（例: 編集アプリ A と閲覧アプリ B）はスキーマとロール定義を共有する
- スイートが定義するもの: テーブルスキーマ、ロール DAG、テーブル権限、マイグレーション
- SuiteID は衝突を避けるため逆ドメイン等を推奨（例: `jp.example.home-visit-suite`）

### ネットワークインスタンス

スイート内の具体的なメンバー集合とデータ空間。ユーザーが実行時に作成する。

- ネットワークインスタンス ID（UUID）で識別される
- メンバーの追加・脱退・ロール割り当てはインスタンス単位
- データ（テーブルの行）はインスタンス単位で分離される
- 1つの DID が同じスイートの複数インスタンスに所属できる

### AppID

アプリ固有の識別子（逆ドメイン。例: `jp.example.home-visit-editor`）。

- 同じスイートの異なるアプリは異なる AppID を持つ
- 同期データは SuiteID + ネットワークインスタンス単位なので AppID の役割は薄い
- 将来的拡張のため概念として残す

### 例: 訪問看護スイート

```
SuiteID: "jp.example.home-visit-suite"（開発者がハードコード）

アプリ A: 訪問記録編集アプリ（AppID: "jp.example.home-visit-editor"）
アプリ B: 訪問記録閲覧アプリ（AppID: "jp.example.home-visit-viewer"）

ネットワークインスタンス:
  ├── 東京クリニック（ID: "550e8400-..."、メンバー: 田中, 佐藤, 山田）
  └── 大阪クリニック（ID: "7c9e6679-..."、メンバー: 鈴木, 高橋, 田中）

→ 田中は両方のインスタンスに所属
→ アプリ A, B は同じ SuiteID を持つため、同じインスタンス内でデータを共有
→ 東京と大阪のデータは完全に分離される
```

---

## 4. LinkSelf のストレージプロキシモデル

### アーキテクチャ

```
┌─────────────────────────────────────────────────────────┐
│ アプリ                                                    │
│  SQL クエリを発行するだけ                                   │
│  （同期・権限の詳細を意識しない）                             │
├─────────────────────────────────────────────────────────┤
│ LinkSelf ストレージプロキシ                                 │
│  ┌─────────────────────────────────────────────────┐     │
│  │ クエリインターフェース                               │     │
│  │  CREATE TABLE / SELECT / INSERT / UPDATE / DELETE │     │
│  │  WHERE / JOIN / INDEX                             │     │
│  └──────────────┬──────────────────────────────────┘     │
│                 │                                        │
│  ┌──────────────▼──────────────────────────────────┐     │
│  │ 権限レイヤー                                      │     │
│  │  このメンバーはこのテーブルを読める/書けるか？         │     │
│  └──────────────┬──────────────────────────────────┘     │
│                 │                                        │
│  ┌──────────────▼──────────────────────────────────┐     │
│  │ 同期エンジン（透過的）                              │     │
│  │  MyDB: 同一DID全デバイス同期                        │     │
│  │  SharedDB: ネットワークメンバー間同期                 │     │
│  │  競合解決・差分同期・オフライン対応                    │     │
│  └──────────────┬──────────────────────────────────┘     │
│                 │                                        │
│  ┌──────────────▼──────────────────────────────────┐     │
│  │ SQLite ストレージ                                  │     │
│  │  実際のデータ永続化                                  │     │
│  └─────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────┘
```

### アプリから見た体験

アプリは SQLite と同じ感覚でデータにアクセスする:

```go
// テーブル作成
client.MyDB().Exec(ctx, `CREATE TABLE IF NOT EXISTS visit_records (
    id TEXT PRIMARY KEY,
    patient_id TEXT NOT NULL,
    date TEXT NOT NULL,
    body TEXT
)`)

// データ書き込み（裏側で自動同期される）
client.MyDB().Exec(ctx, `INSERT INTO visit_records (id, patient_id, date, body)
    VALUES (?, ?, ?, ?)`, id, patientID, date, body)

// リッチなクエリ（WHERE, JOIN, ORDER BY すべて使える）
rows := client.MyDB().Query(ctx, `SELECT * FROM visit_records
    WHERE patient_id = ? AND date > ?
    ORDER BY date DESC`, patientID, sinceDate)
```

アプリは同期を一切意識しない。別のデバイスで書き込まれたデータも、権限があればクエリ結果に含まれる。

---

## 5. MyDB（公開 API）と内部同期メカニズム

**MyDB はアプリが使う唯一の公開 API** である。アプリは `client.MyDB()` を通じて SQL クエリを発行し、データの同期は LinkSelf が透過的に処理する。

内部では **DeviceSync**（同一ユーザーのデバイス間同期）と **SharedDB**（ネットワークメンバー間同期）が協調して動作するが、アプリからこれらの区分は見えない。同期範囲はテーブル単位の権限設定（§6.5）で自動的に決まる。

### アプリから見たデータの見え方

```
アプリが visit_records テーブルにクエリを発行する
  ↓
LinkSelf プロキシが権限を確認する
  - この DID は visit_records テーブルを読めるか？
  ↓
権限がある全データを返す
  - 自分が書いたデータも他メンバーのデータも区別なく返る
```

### 書き込みのフロー

```
アプリが INSERT を実行する
  ↓
LinkSelf プロキシが権限を確認する
  - この DID は visit_records テーブルに書けるか？
  ↓
ローカル SQLite に書き込む
  ↓ 自動的に（プロキシ内部）
同一 DID の全デバイスに同期
  ↓ テーブルの権限設定に応じて
ネットワークメンバーにも配信（read が self 以外の場合）
  ↓
各メンバーのデバイスでクエリ結果に反映される
```

権限と同期範囲の関係は §6.5 を参照。

### 同期スコープの変更（個人→共有への昇格）

テーブルの同期スコープを変更することで、個人データを共有データに昇格できる:

```go
// 個人メモアプリ → 勉強グループで共有
client.MyDB().SetSyncScope(ctx, "notes", linkself.ScopeNetwork, networkID,
    linkself.WithIncludeExisting(true))  // 既存データも共有するか
```

**推奨パターン:** 個人用アプリから共有アプリへの移行は、テーブルスキーマのマイグレーションと合わせて行い、同期スコープ・ロール・権限を設計し直すこと。気軽なトグルではなく、アプリのバージョンアップとして意識的に行う。

`IncludeExisting` オプション:
- `true`: スコープ変更時に既存データもネットワークメンバーに同期する
- `false`（デフォルト）: スコープ変更後の新規書き込みのみ共有対象

### 5.1 ユーザー鍵とデバイス鍵の2層構造

各デバイスは**固有のデバイス鍵（デバイス DID）**を持ち、ネットワークに公開される**ユーザー鍵（ユーザー DID）**は全デバイスで共有する。

```
ユーザー DID (did:key:USER)  ← ネットワークに公開、不変
  ├─ スマホ  (did:key:dev1) ← デバイス固有、ペアリングにのみ使用
  ├─ PC     (did:key:dev2)
  └─ タブレット (did:key:dev3)
```

**ペアリングプロトコル:**
1. 既存デバイスで `CreatePairingToken()` を呼ぶ → 時間制限付きトークン生成
2. アプリがトークンを QR コード等で表示
3. 新デバイスでトークンを入力 → 既存デバイスに接続
4. 既存デバイスがユーザー鍵を暗号化して転送
5. 新デバイスが同一ユーザーとして動作開始（既存データの全同期を実行）

**設計原則:**
- ユーザー鍵の転送はペアリング時の1回のみ。以後のデバイス間認証はデバイス鍵で行う
- ネットワークメンバーからはユーザー DID のみ見える。デバイスの追加・削除は外部に影響しない
- 1台紛失した場合、他デバイスからそのデバイスのペアリングを解除できる（ユーザー鍵の再生成は不要）
- 全デバイス同時消失時はユーザー鍵を失い、データは復元不可能

---

## 6. 権限モデル

アプリが意識する唯一のこと: **ネットワーク内での権限**。

### 6.1 ロールの定義

ロールはアプリが定義する。LinkSelf は「ロールとは何か」を知らない。アプリが起動時にロール定義を登録する。

```go
client.Start(ctx, linkself.Config{
    SuiteID: "jp.example.home-visit-suite",
    Roles: linkself.RoleDefs{
        "viewer":     {},                              // 基本ロール
        "nurse":      {Includes: []string{"viewer"}},  // viewer の権限を含む
        "accountant": {Includes: []string{"viewer"}},  // viewer の権限を含む（nurse とは並列）
        "admin":      {Includes: []string{"nurse", "accountant"}}, // 両方の権限を含む
    },
})

// ネットワークインスタンスの選択は起動後に行う
instances, _ := client.Network().List(ctx)        // 所属するインスタンス一覧
client.Network().Select(ctx, instances[0].ID)     // 使用するインスタンスを選択
```

ロールの関係は有向非巡回グラフ（DAG）であり、線形とは限らない:

```
        admin
       /     \
    nurse   accountant
       \     /
       viewer
```

**設計方針:**
- ロール定義はアプリが Config で渡す。LinkSelf がロールを発明することはない
- ロールの包含関係はアプリが `Includes` で定義する。複数のロールを包含できる（DAG）
- `write = 'nurse'` の場合、nurse を直接または間接的に包含するロール（admin）も書き込み可能
- ロール定義はスイート単位。同じ SuiteID を持つアプリは同じロール定義を共有する
- ロール定義は同期の対象外（アプリのコードに埋め込まれる静的な定義）
- 循環参照（A includes B, B includes A）はエラーとする
- 1メンバー = 1ロール。複数ロールの兼任が必要な場合は、DAG で複合ロールを定義する（例: `nurse_accountant` が nurse と accountant を包含）

### 6.2 メンバーへのロール割り当て

メンバーにロールを割り当てる操作はアプリが行う。

```go
// メンバーにロールを割り当て（選択中のネットワークインスタンスに対して）
client.Network().SetMemberRole(ctx, memberDID, "nurse")

// メンバーのロールを取得
role, _ := client.Network().GetMemberRole(ctx, memberDID)
```

- 1メンバーに割り当てられるロールは1つのみ
- ロール割り当て情報はネットワークメタデータの一部として同期される
- ロールを割り当てる権限自体もアプリが定義できる（例: admin のみがロールを変更可能）
- ロール未割り当てのメンバーは最低限の権限（`members` 権限のみ）を持つ

### 6.3 テーブル単位の権限

テーブル作成時に、どのロールが何をできるかを定義する。行レベルのアクセス制御は LinkSelf では提供せず、アプリが WHERE 句で行う。

```go
// テーブル作成（標準 SQL）
client.MyDB().Exec(ctx, `CREATE TABLE visit_records (
    id TEXT PRIMARY KEY,
    patient_id TEXT NOT NULL,
    date TEXT NOT NULL,
    body TEXT
)`)

// 権限設定（別 API）
client.MyDB().SetPermissions(ctx, "visit_records", linkself.Permissions{
    Read:   "viewer",   // viewer 以上が読める（nurse, admin も含む）
    Write:  "nurse",    // nurse 以上が書き込み可（admin も含む）
    Delete: "owner",    // 書き込んだ本人のみ削除可
})
```

ロールの包含関係が効くため、`Write: "nurse"` とすれば nurse を包含する `admin` も自動的に書き込み可能になる。`accountant` は nurse と並列なので書き込みできない。

### 6.4 権限の種類

| 権限 | 意味 |
|------|------|
| `members` | ネットワークメンバー全員（ロール不問） |
| `owner` | レコードを作成したメンバーのみ。「誰が書いたか」は LinkSelf が内部メタデータとして管理する |
| `<role_name>` | 指定ロール以上を持つメンバー（包含関係を含む） |
| `self` | 自分（この DID）のデータのみ（同期範囲も自デバイスのみ） |

### 6.5 権限と同期の関係（§5 から参照）

テーブルの `read` 権限が同期範囲を決定する:

| read 権限 | 同期範囲 |
|-----------|---------|
| `self` | 同一 DID のデバイス間のみ |
| `<role_name>` | 該当ロール以上のメンバーにのみ配信 |
| `members` | ネットワークメンバー全員に配信 |

`write` / `delete` 権限は同期範囲に影響しない（データは読める人全員に届くが、書き込み・削除は権限があるメンバーのみ実行可能）。

---

## 7. 現行実装の位置づけ

現行の内部実装はストレージプロキシの裏側のメカニズムとして活用する。

| 現行実装 | コンセプト上の位置づけ |
|---------|---------------------|
| `devicesync` パッケージ | DeviceSync エンジン（同一ユーザーの全デバイス間の透過同期） |
| `groupshare` パッケージ | SharedDB エンジン（ネットワークメンバー間の同期）。内部メカニズム |
| `network` パッケージ | ネットワーク管理（メンバー・ロール） |
| `sqlproxy` パッケージ | MyDB の公開 API 実装。SQL → ローカル SQLite 実行 → 同期トリガー |
| ReplicationEngine + ChangeLog | 差分同期・競合解決の内部メカニズム（そのまま維持） |
| SQLite3 ストレージ実装 | データ永続化（LinkSelf が完全内包） |

アプリからはこれらの内部構造は見えない。アプリは `client.MyDB()` で SQL クエリを発行するだけである。

---

## 8. 具体例: 訪問看護スイート

```
スイート「訪問看護スイート」（SuiteID: jp.example.home-visit-suite）
└── ネットワーク「東京クリニック」（インスタンス ID: 550e8400-...）

  メンバー田中（ユーザーDID: did:key:z6Mk...、ロール: nurse）
    ├── Windows PC（デバイスDID: did:key:z6Da...）— 編集アプリ A
    ├── Mac（デバイスDID: did:key:z6Db...）— 編集アプリ A
    ├── iOS（デバイスDID: did:key:z6Dc...）— 閲覧アプリ B
    └── Android（デバイスDID: did:key:z6Dd...）— 閲覧アプリ B

  メンバー佐藤（DID: did:key:z6Nk...、ロール: nurse）
    └── Windows PC — 編集アプリ A

  メンバー山田（DID: did:key:z6Ok...、ロール: admin）
    └── Mac — 管理アプリ C
```

- 田中が Windows PC の編集アプリ A で訪問記録を INSERT する
- 田中の Mac、iOS、Android にも自動的に同期される
- 佐藤のデバイスにも自動的に同期される
- 山田は admin ロール（nurse を包含）なので読み書き両方可能
- **全員が同じ `SELECT * FROM visit_records` で全データにアクセスできる**（ロールの範囲内で）
- アプリは同期の仕組み（DeviceSync / SharedDB）を一切意識しない
- 田中の各デバイスは固有のデバイス DID を持つが、ネットワークからはユーザー DID（did:key:z6Mk...）のみが見える
- 田中が大阪クリニックのネットワークにも所属している場合、アプリ上でインスタンスを切り替えて使う

---

## 9. 永続化方針への反映

[linkself-data-persistence-plan.md](linkself-data-persistence-plan.md) のディレクトリ構造を NetworkID に対応させる:

```
LinkSelf データルート/
└── <DID空間>/
    ├── identity.json
    ├── contacts                      ... DID 共通
    ├── suites/
    │   └── <suite-id>/               ... スイート単位
    │       └── networks/
    │           └── <instance-id>/    ... ネットワークインスタンス単位のデータ
    │               └── data.db       ... アプリのテーブル + 同期メタデータ（単一 DB）
    └── apps/                         ... 将来的拡張用
        └── <app-id>/                 ... アプリ固有設定
```

アプリから見える単一の DB として `data.db` にすべてのテーブルを格納する。MyDB / SharedDB の区分は内部のメタデータ（同期対象・権限設定）で管理し、物理的な DB ファイルは分けない。

---

## 10. Config への反映

```go
type Config struct {
    IdentityPath   string
    SuiteID        string           // アプリ開発者がハードコード（スイートの識別）
    Roles          RoleDefs         // スイートのロール定義（DAG）。アプリがハードコード
    ListenAddrs    []string
    BootstrapPeers []string
    ChangeLogRetention *ChangeLogRetention // ChangeLog 保持ポリシー（nil = デフォルト）
}

type ChangeLogRetention struct {
    Mode     RetentionMode   // TimeBased（デフォルト）| CountBased
    Duration time.Duration   // TimeBased のデフォルト: 30日
    MaxCount int             // CountBased のデフォルト: 10000
}
```

ストレージのパスは LinkSelf が自動決定する。アプリは指定しない。

ネットワークインスタンスの選択は Config ではなく、起動後の API で行う:

```go
// 起動
client.Start(ctx, config)

// 所属するネットワークインスタンス一覧
instances, _ := client.Network().List(ctx)

// インスタンスを選択（以後の MyDB 操作はこのインスタンスに対して行われる）
client.Network().Select(ctx, instanceID)

// 新規作成
newID, _ := client.Network().Create(ctx, memberDIDs)
```

### デバイスのペアリング

新しいデバイスの追加はペアリング API で行う:

```go
// 既存デバイスで: ペアリングトークン生成（時間制限付き）
token, _ := client.CreatePairingToken(ctx, 5 * time.Minute)
// → token をQRコード等でアプリが表示

// 新デバイスで: トークンを使ってペアリング
client.PairWithToken(ctx, token)
// → ユーザー鍵が安全に転送され、以後は同一ユーザーとして同期
```

ペアリングの詳細は §5.1 を参照。

---

## 11. 実装ロードマップ

**実装済み:** 用語整理、Role DAG、Network Service、Permission、MyDB (KV)、SharedDB、DB (SQL)、SubAnnouncement 再接続

| Phase | 内容 | スコープ | 依存 |
|-------|------|---------|------|
| **A** | ChangeLog 保持ポリシー + 全同期フォールバック | DeviceStorage, ReplicationEngine, Config | なし |
| **B** | API 統合（MyDB を唯一の公開 API に。DB()/SharedDB() 廃止） | pkg/linkself/, daemon RPC | なし |
| **C** | SQL-同期接続 + テーブル同期スコープ設定 | sqlproxy → DeviceSync/GroupShare 連携 | B |
| **D** | Config.SuiteID + ストレージ自動配置 | Config, dataroot | B |
| **E** | ユーザー鍵/デバイス鍵の2層構造 + ペアリングプロトコル | did, pairing(新規), node | A, B |

**推奨着手順序:** A と B を並行 → C → D と E を並行
**TDD:** 各ステップは Red→Green→Refactor サイクル。詳細は [実装プラン](../../../.claude/plans/linkself-spec-implementation.md) 参照。

---

## 12. 設計決定・検討事項

設計決定の記録、検討事項、仕様精査結果は [data-sync-decisions.md](data-sync-decisions.md) に分離した。

主要な決定事項のサマリ:
- MyDB が唯一の公開 API（SQL 対応）。DB() と SharedDB() は廃止
- テーブル単位の同期スコープ。read 権限が同期範囲を自動決定
- ユーザー鍵 + デバイス鍵の2層構造。QR ペアリングでユーザー鍵を転送
- ChangeLog 保持ポリシー（時間/件数ベース）。不足時は自動全同期
