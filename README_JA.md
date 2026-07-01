# ikik-api

![Go](https://img.shields.io/badge/Go-1.26.2-00ADD8?logo=go&logoColor=white)
![Vue](https://img.shields.io/badge/Vue-3-42b883?logo=vuedotjs&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-4169E1?logo=postgresql&logoColor=white)
![Redis](https://img.shields.io/badge/Redis-7+-DC382D?logo=redis&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)
![License](https://img.shields.io/badge/License-LGPL--3.0-blue)

ikik-api は Sub2API をベースに二次開発された、セルフホスト向けの AI API ゲートウェイ兼サブスクリプション管理プラットフォームです。アカウントプール、API Key 管理、複数プロバイダーへのリクエスト転送、利用量計測、サブスクリプション課金、モデレーション制御、管理運用機能を提供します。

[English](README.md) | [中文](README_CN.md) | 日本語

このリポジトリは、プライベートデプロイ、カスタマイズ、二次開発を目的としています。本番環境のシークレット、プライベートサーバー設定、ホスティングサービスの認証情報、商用運用データは含まれていません。

## 重要な注意事項

デプロイまたは運用する前に、以下を必ず確認してください。

- 利用規約上のリスク：サブスクリプション型またはアカウント型の上流サービスを経由してリクエストを転送すると、一部の上流プロバイダーの利用規約に違反する可能性があります。使用前に関連する契約を確認してください。
- コンプライアンス：このプロジェクトは、利用する国または地域の法律・規制に従って使用してください。
- アカウントリスク：アカウント停止、クォータリセット、サービス中断、上流ポリシー変更、課金エラーは、運用者が対応すべき運用リスクです。
- 免責事項：このプロジェクトは技術学習、セルフホスティング、二次開発のために提供されています。デプロイ、データ、ユーザー、決済、上流アカウントについては利用者自身が責任を負います。

## 機能

- chat、responses、models、embeddings、image、ストリーミング用途に対応した OpenAI 互換ゲートウェイエンドポイント。
- OpenAI 互換チャネルとアカウント型上流サービスに対応する複数プロバイダーのルーティング。
- 公開、プライベート、所有、相乗り型スケジューリング概念を含むアカウントプール管理。
- グループルーティング、クォータ制御、利用記録、課金メタデータを備えた API Key 管理。
- ユーザーサブスクリプション、チャージフロー、引換コード、招待報酬、ショップ/カードキー機能。
- ユーザー、アカウント、チャネル、決済、モデレーション、リスクイベント、データ管理、システム設定を管理する管理ダッシュボード。
- リクエスト/レスポンス監査のためのコンテンツモデレーションとリスク制御の統合ポイント。
- タグビルド、Docker イメージ、アーカイブ、GitHub Releases に対応した組み込みリリースワークフロー。
- Vue 3、TypeScript、Pinia、Vue Router、Tailwind CSS、Vite によるフロントエンドコンソール。
- Go、Gin、Ent、PostgreSQL、Redis とモジュール化されたサービス境界によるバックエンド。

## 技術スタック

- バックエンド：Go 1.26.2、Gin、Ent、PostgreSQL、Redis
- フロントエンド：Vue 3、TypeScript、Vite、Pinia、Tailwind CSS
- テスト：Go test、Vitest、vue-tsc、ESLint
- デプロイ：Docker またはソースビルド。外部 PostgreSQL と Redis の利用を推奨

## リポジトリ構成

```text
.
├── backend/              # Go バックエンド、マイグレーション、サービス、ハンドラー、リポジトリ
├── frontend/             # Vue 3 管理/ユーザーコンソール
├── deploy/               # デプロイ例と設定テンプレート
├── docs/                 # 追加の連携・運用ドキュメント
├── assets/               # 静的プロジェクトアセット
├── Makefile              # 共通のビルド・テスト入口
└── Dockerfile            # 本番イメージビルド
```

## 要件

- Go 1.26.2
- Node.js 20+
- pnpm 9+
- PostgreSQL
- Redis
- Docker（任意。ただしデプロイでは推奨）

## バージョンと更新

現在のソースバージョンは `1.0.1` です。バージョンファイルは `backend/cmd/server/VERSION` にあり、release tag の公開時にリリースワークフローによって更新されます。

正式リリースでは、GoReleaser によってバックエンドバイナリ、フロントエンドアセット、アーカイブ、Docker イメージ、マルチアーキテクチャ manifest がビルドされます。Docker イメージには正確なバージョンタグと、設定済みの `latest` などのローリングタグが付与されます。

ユーザーおよび運用者向けのバージョン履歴とアップグレードノートは [CHANGELOG_JA.md](CHANGELOG_JA.md) を参照してください。

## 設定

サンプル設定から開始します。

```bash
cp deploy/config.example.yaml deploy/config.yaml
```

生成された設定を環境に合わせて編集します。

- `server`：ホスト、ポート、フロントエンド URL、リクエストボディ制限、CORS、セキュリティヘッダー。
- `database`：PostgreSQL 接続設定。
- `redis`：キャッシュおよびキューバックエンド設定。
- `gateway`：上流タイムアウト、ボディサイズ制限、ルーティング、モデル挙動。
- `security`：URL allowlist、レスポンスヘッダーのフィルタリング、プロキシフォールバック、CSP。
- 必要に応じて payment、email、storage、moderation、OAuth セクションを設定します。

本番認証情報をコミットしないでください。ローカルおよびデプロイ固有の設定ファイルは git で無視されます。

## 開発

フロントエンド依存関係をインストールします。

```bash
pnpm --dir frontend install
```

フロントエンド開発サーバーを起動します。

```bash
pnpm --dir frontend run dev
```

ソースからバックエンドを実行します。

```bash
cd backend
go run ./cmd/server
```

初回起動時、有効な設定またはインストール状態が検出されない場合、バックエンドはセットアップフローを開始することがあります。

## ビルド

バックエンドとフロントエンドをビルドします。

```bash
make build
```

バックエンドのみをビルドします。

```bash
make build-backend
```

フロントエンドのみをビルドします。

```bash
make build-frontend
```

Docker イメージをビルドします。

```bash
docker build -t ikik-api:local .
```

## テスト

設定済みのチェックをすべて実行します。

```bash
make test
```

バックエンドテストを実行します。

```bash
cd backend
go test -tags=unit ./...
go test -tags=integration ./...
```

フロントエンドチェックを実行します。

```bash
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend run i18n:audit:strict
pnpm --dir frontend exec vitest run
```

リポジトリ設定で golangci-lint を実行します。

```bash
cd backend
golangci-lint run ./... --timeout=30m
```

ローカルに `golangci-lint` がない場合は、CI と同じバージョンを使用できます。

```bash
cd backend
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9.0 run ./... --timeout=30m
```

## デプロイメモ

本番環境では、ikik-api を Nginx、Caddy、マネージドロードバランサーなどのリバースプロキシの背後で実行することを推奨します。

### Nginx リバースプロキシに関する注意

Nginx を使用し、アカウントスケジューリング、sticky session、Codex CLI、またはアンダースコアを含むヘッダーを送信するクライアントを利用する場合は、Nginx の `http` ブロックで以下を有効化してください。

```nginx
underscores_in_headers on;
```

Nginx はデフォルトでアンダースコアを含むヘッダーを破棄します。これにより、セッションルーティングや一部のネイティブクライアント互換パスが壊れる可能性があります。

推奨される本番環境の基本事項：

- PostgreSQL と Redis はアプリケーションコンテナの外部で運用します。
- シークレットをイメージに焼き込まず、本番設定ファイルをマウントします。
- TLS はリバースプロキシまたはロードバランサーで終端します。
- `/api/*`、`/v1/*`、ストリーミング、ゲートウェイルートを CDN キャッシュ対象にしないでください。
- リバースプロキシとバックエンドでリクエストボディ制限を一致させます。
- マイグレーションまたはアプリケーションアップグレード前に PostgreSQL をバックアップしてください。

## セキュリティ

- API Key、OAuth secret、決済キー、データベースパスワード、サーバー認証情報をコミットしないでください。
- サービスを公開する前に `deploy/config.example.yaml` を確認してください。
- 強力なパスワード、利用可能であれば MFA、信頼できるリバースプロキシルールで管理画面へのアクセスを制限してください。
- 決済、ストレージ、モデレーション、メール認証情報には最小限の権限のみを付与してください。
- 変更を公開する前に `make secret-scan` を実行してください。

## ライセンス

このプロジェクトは [LICENSE](LICENSE) に含まれるライセンスに従います。

## 謝辞

ikik-api は Sub2API をベースに構築され、セルフホスト AI ゲートウェイ、サブスクリプション、会計、運用ワークフロー向けに拡張されています。
