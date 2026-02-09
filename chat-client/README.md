# LinkSelf Chat Client

LINEライクなチャットクライアント。LinkSelf P2Pインフラを使用したElectronアプリケーション。

## 技術スタック

- **Electron**: デスクトップアプリフレームワーク
- **React**: UIライブラリ
- **TypeScript**: 型安全なコード記述
- **Vite**: ビルドツール
- **Go**: LinkSelf daemon（P2P通信）

## セットアップ

### 前提条件

- Node.js 18以上
- Go 1.24.6以上（coreモジュールの要件）
- Windows環境（この段階ではWindowsを優先）

### インストール

```bash
# 依存関係のインストール
npm install

# LinkSelf daemonのビルド
npm run build:daemon
```

## 開発

```bash
# 開発モードで起動（Vite + Electron）
npm run dev
```

## ビルド

```bash
# TypeScriptコンパイル + Reactビルド
npm run build

# LinkSelf daemonのビルド（Windows用）
npm run build:daemon
```

## プロジェクト構造

```
chat-client/
├── src/
│   ├── main.ts              # Electronメインプロセス
│   ├── preload.ts           # セキュアAPIブリッジ
│   ├── types/
│   │   └── linkself.d.ts    # LinkSelf API型定義
│   └── renderer/
│       ├── main.tsx          # Reactエントリー
│       ├── App.tsx           # メインAppコンポーネント
│       ├── components/       # Reactコンポーネント
│       ├── hooks/            # カスタムフック
│       └── styles/           # CSS
└── build/                    # ビルド成果物（daemonはcore/cmd/linkself-daemonからビルド）
```

## 使用方法

1. アプリを起動すると、自動的にLinkSelfノードが開始されます
2. 自分のDIDがヘッダーに表示されます
3. 連絡先を追加してチャットを開始できます

## テスト動作

**テスト動作させたいとき**は [docs/entry-point.md](docs/entry-point.md) を起点に手順を進めてください。詳細は [docs/testing-guide.md](docs/testing-guide.md) を参照。

### クイックスタート

```bash
# 1. 依存関係のインストール
npm install

# 2. LinkSelf daemonのビルド
npm run build:daemon

# 3. 開発モードで起動
npm run dev
```

## 注意事項

- この段階では**モック表示まで**。実際のP2P通信は後続フェーズで実装
- identityは `~/.linkself/identity.json` に保存されます
- Windows環境での動作を優先（他のプラットフォームは後続）

## ドキュメント

- [テスト動作のエントリポイント](docs/entry-point.md) — テスト動作させたいときはここから
- [実装計画](docs/implementation-plan.md)
- [テスト動作ガイド](docs/testing-guide.md)
- [友達追加・申請承認・複数インスタンス](docs/friend-add-and-multi-instance.md)
