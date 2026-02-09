# Electronチャットクライアント（モック）実装計画

## 概要

`chat-client`フォルダをルートとして、Electronアプリケーションを作成。LinkSelfライブラリをSubprocess方式（Goバイナリを子プロセスとして起動し、JSON-RPCで通信）で統合し、Windows環境でLINEライクなUIモックを表示する。

## アーキテクチャ

```
chat-client/
├── package.json              # Electron + React + TypeScript + 依存関係
├── tsconfig.json             # TypeScript設定
├── vite.config.ts            # Vite設定（Reactビルド用）
├── src/
│   ├── main.ts               # Electronメインプロセス（LinkSelf daemon管理）
│   ├── preload.ts            # レンダラープロセスへの安全なAPI公開
│   └── types/
│       └── linkself.d.ts     # LinkSelf daemon API型定義
├── src/renderer/
│   ├── index.html            # Reactアプリのエントリーポイント
│   ├── main.tsx              # Reactアプリのエントリー（ReactDOM.render）
│   ├── App.tsx               # メインAppコンポーネント
│   ├── components/
│   │   ├── ChatWindow.tsx    # チャットウィンドウコンポーネント
│   │   ├── MessageList.tsx  # メッセージリストコンポーネント
│   │   ├── MessageBubble.tsx # メッセージバブルコンポーネント
│   │   ├── MessageInput.tsx  # メッセージ入力コンポーネント
│   │   └── ContactList.tsx  # 連絡先一覧コンポーネント
│   ├── hooks/
│   │   └── useLinkSelf.ts    # LinkSelf API用カスタムフック
│   ├── styles/
│   │   └── App.css           # LINEライクなスタイル
│   └── types.ts              # 型定義
└── build/                    # ビルド成果物（Goバイナリ、バンドル済みJS等）

**注意**: daemonは`core/cmd/linkself-daemon/main.go`に配置されています（`core`モジュールの`internal`パッケージにアクセスするため）。
```

## 実装タスク

### 1. Electronアプリの基本構造（TypeScript + React）

- **package.json**: Electron、React、TypeScript、Vite、必要な依存関係を定義
- **tsconfig.json**: TypeScriptコンパイラ設定（JSX対応）
- **vite.config.ts**: Vite設定（Reactビルド、Electron統合）
- **src/main.ts**: 
  - Electronウィンドウの作成
  - LinkSelf daemon（Goバイナリ）の起動・管理
  - daemonとのJSON-RPC通信（stdio経由）
  - IPCハンドラー（レンダラー ↔ メインプロセス）
- **src/preload.ts**: セキュアなAPIブリッジ（`contextBridge.exposeInMainWorld`）
- **src/types/linkself.d.ts**: JSON-RPC APIの型定義

### 1-2. React UI層

- **src/renderer/index.html**: Reactアプリのエントリーポイント
- **src/renderer/main.tsx**: Reactアプリのエントリー（ReactDOM.createRoot）
- **src/renderer/App.tsx**: メインAppコンポーネント（ルーティング、レイアウト）
- **src/renderer/components/ChatWindow.tsx**: チャットウィンドウコンポーネント
- **src/renderer/components/MessageList.tsx**: メッセージリストコンポーネント（スクロール可能）
- **src/renderer/components/MessageBubble.tsx**: メッセージバブルコンポーネント（送信/受信のスタイル分岐）
- **src/renderer/components/MessageInput.tsx**: メッセージ入力コンポーネント（送信ボタン含む）
- **src/renderer/components/ContactList.tsx**: 連絡先一覧コンポーネント
- **src/renderer/hooks/useLinkSelf.ts**: LinkSelf API用カスタムフック（IPC通信、状態管理）
- **src/renderer/styles/App.css**: LINEライクなスタイル（メッセージバブル、タイムライン等）
- **src/renderer/types.ts**: アプリ内で使用する型定義（Message, Contact等）

### 2. LinkSelf Daemon（Go）

- **core/cmd/linkself-daemon/main.go**（daemon は core 内に配置。chat-client に daemon ディレクトリはない）:
  - JSON-RPCサーバー（stdio経由）
  - LinkSelf Nodeの初期化・管理（pkg/linkself のみ依存）
  - メソッド: `start`, `stop`, `getMyDID`, `sendMessage`, `connect`, `setOnMessage`等
  - identityの保存・読み込み（`identity.json`）
  - メッセージ受信時のJSON-RPC通知

### 3. UIモック（React + LINEライク）

- **チャット画面（Reactコンポーネント）**:
  - `MessageList`: メッセージリスト（送信/受信メッセージのバブル表示、スクロール対応）
  - `MessageBubble`: 個別メッセージバブル（送信=緑、受信=白、タイムスタンプ表示）
  - `MessageInput`: 入力欄と送信ボタン（Enterキー送信対応）
  - `ContactList`: 連絡先一覧（サイドバーまたは別画面）
  - 自分のDID表示（ヘッダーまたは設定画面）
- **スタイル**: LINEのような緑/白のメッセージバブル、タイムスタンプ表示、レスポンシブレイアウト
- **状態管理**: React Hooks（useState, useEffect）を使用

### 4. 統合と通信フロー

- ElectronメインプロセスがGo daemonを起動
- JSON-RPCでコマンド送信（例: `{"jsonrpc":"2.0","method":"start","params":{...},"id":1}`）
- daemonからの通知をIPCでレンダラープロセスに転送
- Reactコンポーネントが`useLinkSelf`フック経由でIPC通信
- Reactの状態更新によりUIが自動更新

### 5. ビルド設定

- **Vite**: Reactアプリのビルドと開発サーバー
- **TypeScriptコンパイル**: ViteがTSX/TSを自動コンパイル
- **ビルドスクリプト**: 
  - `dev`: Vite開発サーバー + Electron起動
  - `build`: Reactアプリをビルド + Electron用にパッケージング
- **Electron統合**: Viteのビルド成果物をElectronのレンダラープロセスで読み込み

### 6. Windows環境での動作確認

- Goバイナリのビルド（Windows用）
- TypeScriptのコンパイル
- Electronアプリの起動確認
- モックデータでのUI表示確認

## 技術スタック

- **Electron**: デスクトップアプリフレームワーク
- **React**: UIライブラリ（コンポーネントベース）
- **TypeScript**: 型安全なコード記述
- **Vite**: ビルドツール（React + TypeScript対応）
- **Node.js**: メインプロセス・レンダラープロセス
- **Go**: LinkSelf daemon（`core`モジュールを使用）
- **JSON-RPC 2.0**: プロセス間通信プロトコル

## ファイル構成

- [package.json](../package.json): Electron + React + TypeScript + Vite設定と依存関係
- [tsconfig.json](../tsconfig.json): TypeScript設定（JSX対応）
- [vite.config.ts](../vite.config.ts): Vite設定（Reactビルド）
- [src/main.ts](../src/main.ts): Electronメインプロセス（TypeScript）
- [src/preload.ts](../src/preload.ts): セキュアAPIブリッジ（TypeScript）
- [src/types/linkself.d.ts](../src/types/linkself.d.ts): JSON-RPC API型定義
- [src/renderer/index.html](../src/renderer/index.html): Reactアプリエントリーポイント
- [src/renderer/main.tsx](../src/renderer/main.tsx): Reactアプリエントリー
- [src/renderer/App.tsx](../src/renderer/App.tsx): メインAppコンポーネント
- [src/renderer/components/](../src/renderer/components/): Reactコンポーネント群
- [src/renderer/hooks/useLinkSelf.ts](../src/renderer/hooks/useLinkSelf.ts): LinkSelf API用カスタムフック
- [src/renderer/styles/App.css](../src/renderer/styles/App.css): LINEライクなスタイル
- [src/renderer/types.ts](../src/renderer/types.ts): アプリ内型定義
- [core/cmd/linkself-daemon/main.go](../../core/cmd/linkself-daemon/main.go): LinkSelf daemon

## 注意事項

- この段階では**モック表示まで**。実際のP2P通信は後続フェーズで実装
- LinkSelfの既存API（`Connect`, `SendMessage`, `SendToGroup`）を使用
- identityの永続化は`identity.json`ファイルに保存（後でKeychain等に移行可能）
- Windows環境での動作を優先（他のプラットフォームは後続）

## 参照

- [LinkSelf core/internal/node/node.go](../../core/internal/node/node.go): Node API
- [docs/using-linkself-as-library.md](../../docs/using-linkself-as-library.md): ライブラリ利用方針
- [docs/sample-chat-app-plan.md](../../docs/sample-chat-app-plan.md): サンプルアプリ計画

## 実装状況

- ✅ Electronアプリの基本構造（TypeScript + React）
- ✅ React UI層
- ✅ LinkSelf Daemon（Go）
- ✅ UIモック（React + LINEライク）
- ✅ 統合と通信フロー
- ✅ ビルド設定
- ✅ Windows環境での動作確認準備完了
