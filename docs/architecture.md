# Design Doc: Serverless Container + Ephemeral SQLite + Object Storage Backup

- **Author:** Kawabata Shintaro
- **Date:** 2025-10-14
- **Status:** Proposed
- **Last Updated:** 2025-10-15

---

## 1. Overview

- 最大 10 名程度の業務利用（7時-21時）を想定した小規模 Web アプリケーションのアーキテクチャ設計。
- 目標は **ランニングコスト最小化、ベンダーロックインの低減、運用の簡素化**。
- アプリは **サーバレスなコンテナ実行基盤**で稼働し、データは **コンテナ内の一時ローカル領域上の SQLite** に保存。
- 永続化は **オブジェクトストレージ**への**一貫スナップショット**（バックアップ）で実現する。

---

## 2. Goals

- **低コスト**：無料枠〜極小コストでの運用
- **低ロックイン**：標準的な OCI コンテナと SQLite を採用し移設容易
- **運用容易**：OS/VM 管理不要、最小の構成要素
- **必要十分な整合性**：小規模トランザクションでの一貫性確保

### Non-Goals
- 高トラフィック・高同時書き込み・大規模スケーラビリティ
- 高可用性・自動フェイルオーバー（単一リージョン運用前提）
- 大容量のデータ・分析・レポーティング

---

## 3. System Context
```bash
[Browser (SPA)]
|
v
[Serverless Container Runtime] --reads/writes--> [SQLite on Ephemeral Local FS]
|
+--(backup/restore)--> [Object Storage]
```

- Web UI は静的アセットとして配信（例：SPA）
- API は同一コンテナで提供
- データはローカル SQLite で高速・単純に扱い、オブジェクトストレージへバックアップ

---

## 4. Architecture

### 4.1 Components
- **Frontend**: SPA (例：React/Vite) をビルドし静的配信
- **Backend**: HTTP サーバ（例：Go/標準ライブラリ）
- **Database**: SQLite（**コンテナ内の一時ディスク**に配置）
- **Object Storage**: バックアップ保管

### 4.2 Data Flow
1. **起動時**: Object Storage の `current` スナップショットがあればダウンロード、無ければ新規 DB 作成
2. **稼働中**: アプリはローカル SQLite に読み書き
3. **バックアップ**:
   - `VACUUM INTO` / Backup API 等で **一貫スナップショット**を一時ファイルに作成
   - Object Storage に一時名でアップロード → **Object Storage 側コピーで `current` を更新**
   - 併せて `backups/yyyymmdd/...` に世代保管
4. **終了時**: 最新 DB を同様に `current` と `backups/` に反映（可能な範囲）

> **注**: オブジェクトストレージはファイルシステムではないため、**直接 SQLite を置いて開かない**。必ずローカルで開いて、スナップショットで同期する。

---

## 5. Reliability & Consistency

- **WAL モード**：書込と読み取りの安定化
- **一貫スナップショット**：`VACUUM INTO` / Backup API を用い、書込中でも壊れにくいスナップショットを作成
- **二相アップロード**：`tmp-object → copy to current → tmp削除` で切替時の一瞬の不整合を回避
- **バージョニング**：誤削除・破損からの復元性を確保
- **同時書込制御**：実行基盤のインスタンス数は 1、同時実行数も 1桁に制限（小規模要件に整合）

---

## 6. Security & Operations

- **TLS**：実行基盤の終端/マネージド証明書を利用
- **キャッシュ**：
  - `index.html` は `must-revalidate` で毎回再検証
  - 生成物ファイル（ハッシュ名）は `public, max-age=N, immutable`
- **メンテナンス**：環境変数でメンテモード切替（API 503）
- **監視**：構造化ログ

---

## 7. Cost

- 実行基盤：無料枠（短時間起動・低リクエスト）で運用可能
- オブジェクトストレージ：数百 MB〜数 GB 程度で低コスト
- コンテナレジストリ：イメージ数・世代に応じて微小コスト

**概算**：無料〜数百円/月（使用量に依存）

---

## 8. Risks & Mitigations

| リスク | 説明 | 緩和策 |
|---|---|---|
| DB 破損 | 書き込み中の強制停止 | WAL + 一貫スナップショット、二相アップロード、バージョニング |
| 並行書込競合 | 複数インスタンスによる同時書込 | 実行基盤のインスタンス数は 1 に制御、書込系エンドポイントの Busy 時はリトライ |
| 自動削除事故 | ライフサイクル誤設定でバックアップ消滅 | lifecycle 削除を `backups/` のみに限定 |
| コールドスタート | 初回応答遅延 | 0台の時のレスポンスタイムが 1s〜2s であれば許容 |
| 将来のスケール | 同時ユーザ/書込増 | 他のデータストアへの移行を可能な設計にしておく |

---

## 9. Migration Paths (Future)

- **RDB（マネージド）へ**：Repository を介したデータアクセスにしておき、SQLite → Postgres/MySQL へ差替え
- **バックアップ自動化**：Container 起動中に周期実行
- **エッジ実行へ**：読み取り系 API をエッジへ段階移行(書き込みがあった際の SQLite ファイルの同期タイミングは課題)。書込は当面サーバ側に残すハイブリッド

---

## Appendix A. Example Mappings (Non-normative)

> 本文は実装基盤に依存しない抽象設計。下表は代表例の**参考対応**であり、特定ベンダーへの固定を意図しない。

| 抽象コンポーネント | クラウド例 |
|---|---|
| Serverless Container Runtime | Cloud Run / App Runner / Azure Container Apps / AppRun |
| Object Storage | Cloud Storage / S3 / Azure Blob Storage |
| Container Registry | Artifact Registry / ECR / ACR |
