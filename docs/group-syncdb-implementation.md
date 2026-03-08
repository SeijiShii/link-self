# グループ・同期 DB 実装ドキュメント

**日本語**（このページ）| [English](group-syncdb-implementation.en.md)
**参照:** [グループの概念](group-concept.md)、[データ同期設計（DeviceSync / GroupShare）](sync-db-plan.md)

本ドキュメントは、上記計画書と整合する形で**実装の配置・API・テスト**をまとめたものです。

> **注意（2026-03）:** 同期 DB は **DeviceSync / GroupShare 二層アーキテクチャ** に移行した。旧 `syncdb` パッケージ（§3）は `devicesync` + `groupshare` で置換予定。新アーキテクチャの詳細は [sync-db-plan.md](sync-db-plan.md) を参照。

---

## 1. グループ（core/internal/group）

### 1.1 計画との対応

| 計画（group-concept.md） | 実装 |
|--------------------------|------|
| グループ = メンバー DID の集合、最低 2 人 | `Group.Members`、`Service.CreateGroup` で 2 人未満は `ErrTooFewMembers` |
| 1 対 1 = 2 人グループ | 専用型なし。2 人で作成すればよい |
| ストアはインターフェース化 | `Store` インターフェース。呼び出し側はこれにのみ依存 |
| メンバー脱退・2 人グループは解体 | `Service.Leave`。脱退後メンバー 1 人のとき `DeleteGroup` |
| オーナー: キック・任命・自己降格・他オーナー降格不可 | `Service.Kick`, `AppointOwner`, `SelfDemote`, `DemoteOwner`（他オーナーは `ErrCannotDemoteOtherOwner`） |
| 最後のオーナー脱退/降格時は自動昇格 | `autoPromoteOneExcluding` で残りメンバーから 1 人をオーナーに昇格 |

### 1.2 パッケージ構成

| ファイル | 内容 |
|----------|------|
| `store.go` | `Group` 型（ID, Members, Owners）、`Store` インターフェース、`ErrGroupNotFound` |
| `memstore.go` | インメモリ `Store` 実装（UUID で groupID 生成）。テスト・検証用 |
| `group.go` | `Service` とドメインロジック。`ErrTooFewMembers`, `ErrNotMember`, `ErrNotOwner`, `ErrCannotDemoteOtherOwner`, `ErrTargetNotMember` |

### 1.3 主要 API

- **Store**: `ListGroupIDsForMember`, `GetGroup`, `CreateGroup`, `UpdateGroup`, `DeleteGroup`
- **Service**: `NewService(store)`, `CreateGroup`, `Leave`, `Kick`, `AppointOwner`, `SelfDemote`, `DemoteOwner`

### 1.4 テスト（core/internal/group）

**ストア（store_test.go）**

| テスト | 内容 |
|--------|------|
| `TestMemStore_CreateGet` | Create 後に Get で同一内容が返る |
| `TestMemStore_ListByMember` | メンバー DID で所属グループ一覧が取れる |
| `TestMemStore_Update` | Update 後 Get で反映される |
| `TestMemStore_Delete` | Delete 後 Get で nil、List からも消える |
| `TestMemStore_GetNotFound` | 存在しない ID で Get は nil |
| `TestMemStore_CreateReturnsUniqueIDs` | 複数 Create で ID が一意 |

**ドメイン（group_test.go）**

| テスト | 内容 |
|--------|------|
| `TestCreateGroup_RequiresAtLeastTwoMembers` | 1 人・nil で作成失敗 |
| `TestCreateGroup_SucceedsWithTwoOrMore` | 2 人以上で成功。2 人でオーナー 0 も許容 |
| `TestLeave_RemovesMember` | 3 人グループで 1 人脱退するとメンバーから外れる |
| `TestLeave_DissolvesTwoMemberGroup` | 2 人グループで 1 人脱退するとグループ削除 |
| `TestLeave_NotMember` | 非メンバーの脱退は `ErrNotMember` |
| `TestOwner_Kick` | オーナーがメンバーをキックできる |
| `TestOwner_KickRequiresOwner` | 非オーナーのキックは `ErrNotOwner` |
| `TestOwner_AppointOwner` | オーナーが他メンバーをオーナーに任命できる |
| `TestOwner_SelfDemote` | オーナーが自己降格できる |
| `TestOwner_CannotDemoteOtherOwner` | 他オーナー降格は `ErrCannotDemoteOtherOwner` |
| `TestLastOwnerLeaving_AutoPromotes` | 最後のオーナーが自己降格すると残りから 1 人昇格 |
| `TestLastOwnerLeaving_OneMemberBecomesOwner` | 2 人グループで最後のオーナー降格時、もう 1 人がオーナーに |
| `TestLastOwnerLeaving_ByLeave` | 最後のオーナーが脱退したときも自動昇格 |

実行: `go test ./internal/group/...`

---

## 2. ノードの SendToGroup（core/internal/node）

### 2.1 計画との対応

- [phase1-design.md](phase1-design.md) / [sync-db-plan.md](sync-db-plan.md): 送信 API はグループ単位（SendToGroup）を想定。

### 2.2 実装

- **SendToGroup(ctx, memberDIDs, payload)**  
  自ノードの `Identity.DID` を memberDIDs から除外し、各 DID に対して既存の `SendMessage(ctx, did, payload)` を呼ぶ。オフライン相手は Store-and-Forward にキューされる。

---

## 3. 同期 DB（core/internal/syncdb）

### 3.1 計画との対応

| 計画（sync-db-plan.md） | 実装 |
|------------------------|------|
| メタ付与・即時配信・後勝ち適用 | `SyncLayer.Put` でメタ付与・保存・SendToGroup。`HandleIncoming` で後勝ち適用 |
| ストレージはインターフェース化 | `RecordStorage` インターフェース。アプリが実装を注入 |
| メタ: ID, groupId, DID, Timestamp | `SyncRecord`: ID, GroupID, DID, Timestamp, Body, Deleted |
| groupId → メンバー DID はグループ概念に従う | `MemberResolver`。実装は `GroupStoreResolver`（group.Store + 自 DID 除外） |
| ペイロードは JSON 等で符号化 | `SyncRecord` を JSON でシリアライズして送受信 |
| 後勝ちは Timestamp 比較 | `GetTimestamp` で既存取得。受信 Timestamp が既存より大きい場合のみ Put |
| 削除の扱い | `SyncRecord.Deleted`。true のとき受信側で `Delete` を適用 |

### 3.2 パッケージ構成

| ファイル | 内容 |
|----------|------|
| `record.go` | `SyncRecord`（ID, GroupID, DID, Timestamp, Body, Deleted）。JSON タグ付き |
| `storage.go` | `RecordStorage` インターフェース（Put, Get, GetTimestamp, Delete）、`ErrNotFound` |
| `memstorage.go` | インメモリ `RecordStorage`。テスト・検証用 |
| `resolver.go` | `MemberResolver`（MemberDIDsForGroup）。`GroupStoreResolver`（group.Store ラップ、自 DID 除外） |
| `sync.go` | `SyncLayer`。`NewSyncLayer(storage, resolver, sendGroup, selfDID)`。`Put`, `PutRecord`, `HandleIncoming` |

### 3.3 主要 API

- **RecordStorage**: `Put`, `Get`, `GetTimestamp`, `Delete`
- **MemberResolver**: `MemberDIDsForGroup(ctx, groupID) ([]string, error)`
- **SyncLayer**: `Put(ctx, groupID, body)` → メタ付与・保存・即時配信。`HandleIncoming(ctx, payload)` → デコード・後勝ちで Put/Delete

### 3.4 テスト（core/internal/syncdb）

| テスト | 内容 |
|--------|------|
| `TestSyncLayer_Put_AttachesMetaAndStores` | Put で ID/DID/Timestamp が付与され、ストレージに保存され、SendToGroup が呼ばれペイロードにメタが含まれる |
| `TestSyncLayer_HandleIncoming_LastWriteWins_NewerApplied` | 新しい Timestamp のレコードが適用される |
| `TestSyncLayer_HandleIncoming_LastWriteWins_OlderSkipped` | 古い Timestamp のレコードは上書きしない |
| `TestSyncLayer_HandleIncoming_DeletedAppliesAsDelete` | Deleted レコードでリモート側が Delete される |

実行: `go test ./internal/syncdb/...`

---

## 4. 統合テスト（core/test/integration）

### 4.1 グループ・同期 DB（syncdb_test.go）

| テスト | 内容 |
|--------|------|
| `TestSyncDB_PutOnA_ReceivedOnB` | 2 ノード（A, B）、1 グループ（group.Store で A/B をメンバーに作成）。各ノードに MemStorage + SyncLayer。B の SetOnMessage で SyncLayer.HandleIncoming を登録。A が Put(groupID, "hello from A") を実行し、B のストレージに同じレコードが反映されることを検証 |

実行: `go test ./test/integration/... -run TestSyncDB`

### 4.2 既存統合テストとの関係

- `TestDIDGenerationConsistency`, `TestDHTProvideFind`, `TestConnectAuth`, `TestMessageOnline`, `TestStoreAndForward` は従来どおり。SendToGroup 追加後もすべてパス。

---

## 5. 計画書との整合チェック

- **group-concept.md**: §1〜§7 の要件（2 人以上、脱退・解体、オーナー権限・他オーナー降格不可、最後のオーナー時の自動昇格、権限はアプリ層）は実装とテストでカバー。
- **sync-db-plan.md**: メタ付与・即時配信・後勝ち適用、ストレージインターフェース、groupId に紐づくメンバー DID（group.Store 経由）、SendToGroup／SetOnMessage の利用は実装済み。SQLite 参照実装・スキーマ規約は未実装（計画どおりアプリ側または別タスク）。

以上で、計画書の内容と実装・テストの整合性をドキュメント化している。
