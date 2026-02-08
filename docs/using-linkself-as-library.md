# LinkSelf をライブラリとして使う

**日本語**（このページ）| [English](using-linkself-as-library.en.md)  
**ステータス:** Phase 2 で整備予定（方針と利用例のみ記載）  
**概要:** LinkSelf（Go コア）を各プラットフォームでライブラリとして利用するための方針と、プラットフォーム別の呼び出し方・利用例。  
**参照:** [Phase 1 設計](phase1-design.md)、[サンプルアプリ計画](sample-chat-app-plan.md)、[ロードマップ](README.ja.md#roadmap)

---

## 方針

- **各環境で利用可能なライブラリ化** — LinkSelf（Go コア）を、各プラットフォームから「ライブラリ」として呼び出せる形にする。公開 API・パッケージング・呼び出し手順は Phase 2 で整備する。
- **プラットフォームに合わせる** — 各プラットフォームに適した UI・呼び出し方で使う。UI フレームワークはプラットフォームごとに選べる（Electron, Flutter, ネイティブなど）。

---

## プラットフォーム別の提供形態（例）

| プラットフォーム | Go の提供形態（例） | UI の選択肢 |
|------------------|----------------------|-------------|
| Windows          | DLL（CGO）+ FFI / または subprocess（stdio/socket） | Electron, WinUI, WPF, Flutter など |
| macOS            | .dylib + FFI / または subprocess | Electron, Swift/SwiftUI, Flutter など |
| Linux            | .so + FFI / subprocess | Electron, GTK, Flutter など |
| Android          | gomobile → AAR        | Kotlin/Compose, Flutter など |
| iOS              | gomobile → Framework  | Swift/SwiftUI, Flutter など |

---

## 利用例（Windows）

### Windows + Electron

- **子プロセス**: Go を `linkself` バイナリ（または専用 daemon）として起動し、Electron のメインプロセスから stdio や TCP/socket で JSON-RPC などで通信する。実装が単純で、Go 側の変更も少ない。
- **Node FFI**: Go を C 共有ライブラリ（DLL）にビルドし、`node-ffi-napi` 等で呼ぶ。プロセスを分けずに呼べるが、CGO ビルド・ABI の整備が必要。

### Windows + Flutter

- Go を C ダイナミックライブラリ（DLL）にビルドし、**dart:ffi** で呼ぶ。または Go をサブプロセスで起動し、stdio/socket で通信する。Phase 2 でビルド手順・FFI バインディング例を整備する予定。

---

## 今後の整備（Phase 2）

- 公開 API の安定化とドキュメント化。
- 各プラットフォーム向けのパッケージング（DLL / AAR / Framework / バイナリ）とビルド手順。
- 上記利用例に沿ったサンプルコードまたは手順の追記。
