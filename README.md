# mini-web-app

## 概要
- Go(HTTP) + 抽象化DataStore（初期はSQLite/WAL）+ 静的ファイル配信
- Cloud Run 単一インスタンス前提（同時実行1）
- 起動時にローカル(`./tmp/app.sqlite`)または`/tmp/app.sqlite`を使用
- ストレージ抽象化: `ObjectStore` 経由でスナップショット同期（GCS/ローカル）
  - GCS: VACUUM INTOで一貫スナップショット→tmp→current二相アップロード＋世代保管
  - ローカル: 終了時のみ `./tmp/backups/` にスナップショット
  - 定期バックアップ（オプトイン、デフォルト無効）

## ディレクトリ構成
- `cmd/server/` エントリポイント
- `internal/`
  - `httpx/` ミドルウェア・静的配信
  - `domain/` モデル・リポジトリIF
  - `infra/`
    - `platform/` ロガー等
    - `datastore/` DBファサード
      - `sqlite/` SQLite実装
    - `storage/` ObjectStore実装
- `frontend/` 静的ファイル
- `scripts/` デプロイ・ビルドスクリプト

## ローカル起動手順
```bash
cp .envrc-example .envrc
direnv allow

go run ./cmd/server
# http://localhost:8080 へアクセス
```

## 主要エンドポイント
- `GET /healthz` ヘルスチェック（DB ping含む）
- `GET /api/singers` 歌手一覧（例）

## デプロイ手順
```bash
# ビルド
./scripts/build.sh

# デプロイ
./scripts/safe_deploy.sh
```

## 運用上の挙動
- 起動時: （GCS利用時）最新DBをダウンロードしローカル配置
- 稼働中: SQLiteはWALモード。`index.html`は`no-cache, max-age=0, must-revalidate`、ハッシュ付きアセットは長期キャッシュ
- 定期バックアップ: デフォルト無効（`PERIODIC_BACKUP=on`で有効化、`PERIODIC_BACKUP_MINUTE`で間隔）。VACUUM INTOで一貫スナップショット
  - GCS: tmp→currentコピー→世代保管
  - ローカル: `./tmp/backups/`に保存
- 終了時: 常にスナップショット取得

## バックアップ設計メモ
- SQLite Online Backup API（`sqlite3_backup_*`）はpure Goドライバ（`modernc.org/sqlite`）では未サポート
- そのためWALモードでも一貫コピー可能な`VACUUM INTO`を採用
- 書き込み競合時はbusy_timeout付き別接続＋短いバックオフリトライ

## 移行容易性
- DB: `internal/infra/datastore.DataStore`・`internal/domain/repository`のIFに依存
  - 他DB移行時は`internal/infra/datastore/<driver>`追加・分岐拡張
- オブジェクトストレージ: `internal/infra/storage.ObjectStore`に依存
  - S3/Blob Storage移行時は`internal/infra/storage/<provider>`追加・起動時切替

## TODO
- テストコードの追加（ユニットテスト・統合テスト）
- CI/CDパイプラインの整備
