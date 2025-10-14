package sqlite

import (
	"context"
	"time"

	storageif "github.com/kawabatas/mini-web-app/internal/infra/storage"
)

// GCSSnapshotStrategy は SQLite のスナップショットを GCS（等のObjectStore）に同期する戦略です。
// - 起動時: currentKey をローカルにダウンロード
// - 終了時: VACUUM INTO で一貫スナップショットを作成 → 二相アップロード + backups/ に保管
type GCSSnapshotStrategy struct {
	ObjectStore storageif.ObjectStore
	Bucket      string
}

func (s GCSSnapshotStrategy) OnStartup(ctx context.Context, dbPath string) error {
	if s.ObjectStore == nil || s.Bucket == "" {
		return nil
	}
	return s.ObjectStore.DownloadIfNeeded(ctx, s.Bucket, FileName, dbPath)
}

func (s GCSSnapshotStrategy) OnShutdown(ctx context.Context, dbPath string) error {
	if s.ObjectStore == nil || s.Bucket == "" {
		return nil
	}
	snap := "/tmp/app-snapshot-" + time.Now().UTC().Format("20060102-150405") + ".sqlite"
	if err := SnapshotTo(ctx, dbPath, snap); err != nil {
		return err
	}
	backupKey := "backups/" + time.Now().UTC().Format("2006-01-02") + "/" + time.Now().UTC().Format("150405") + "-" + FileName
	return s.ObjectStore.UploadTwoPhaseWithBackup(ctx, s.Bucket, FileName, backupKey, snap)
}

// 注: インターフェイス実装の明示は循環参照を避けるため省略
