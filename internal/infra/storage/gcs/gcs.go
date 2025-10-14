package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	storageif "github.com/kawabatas/mini-web-app/internal/infra/storage"
	"github.com/kawabatas/mini-web-app/internal/util/clock"
)

// DownloadIfNeeded fetches object into dest. Creates empty file if not found.
func DownloadIfNeeded(ctx context.Context, bucketName, objectName, dest string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	rc, err := client.Bucket(bucketName).Object(objectName).NewReader(ctx)
	if err != nil {
		// オブジェクトが存在しない場合は空ファイル作成し、nil を返す
		if errors.Is(err, storage.ErrObjectNotExist) {
			slog.WarnContext(ctx, fmt.Sprintf("datastore file is not found on GCS, so create new file: %s", objectName))
			f, cErr := os.Create(dest)
			if cErr == nil {
				f.Close()
			}
			return nil
		}
		return err
	}
	defer rc.Close()

	if err := os.MkdirAll("/tmp", 0755); err != nil {
		return err
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

// Adapter implements storage.ObjectStore using GCS client per call.
// 現状、起動・終了・定期実行（10分間隔など）での使用のみを想定しているため GCS クライアントは都度生成します。
type Adapter struct{}

var _ storageif.ObjectStore = (*Adapter)(nil)

func (a *Adapter) DownloadIfNeeded(ctx context.Context, bucket, object, dest string) error {
	return DownloadIfNeeded(ctx, bucket, object, dest)
}

// UploadTwoPhaseWithBackup implements two-phase publish and versioned backup.
func (a *Adapter) UploadTwoPhaseWithBackup(ctx context.Context, bucket, currentObject, backupObject, localPath string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	// 1. upload to tmp object
	ts := clock.NowUTCFormatted("20060102-150405")
	base := filepath.Base(currentObject)
	tmpName := currentObject + ".tmp-" + ts

	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	wc := client.Bucket(bucket).Object(tmpName).NewWriter(ctx)
	if _, err := io.Copy(wc, f); err != nil {
		_ = wc.Close()
		_ = f.Close()
		return err
	}
	if err := wc.Close(); err != nil {
		_ = f.Close()
		return err
	}
	_ = f.Close()

	// 2. copy tmp -> current
	dst := client.Bucket(bucket).Object(currentObject)
	copier := dst.CopierFrom(client.Bucket(bucket).Object(tmpName))
	if _, err := copier.Run(ctx); err != nil {
		_ = client.Bucket(bucket).Object(tmpName).Delete(ctx)
		return err
	}

	// 3. copy tmp -> backups/yyyy-mm-dd/HHMMSS-<base>
	if backupObject == "" {
		backupObject = "backups/" + clock.NowUTCFormatted("2006-01-02") + "/" + clock.NowUTCFormatted("150405") + "-" + base
	}
	bdst := client.Bucket(bucket).Object(backupObject)
	bcopier := bdst.CopierFrom(client.Bucket(bucket).Object(tmpName))
	if _, err := bcopier.Run(ctx); err != nil {
		_ = client.Bucket(bucket).Object(tmpName).Delete(ctx)
		return err
	}

	// 4. delete tmp
	return client.Bucket(bucket).Object(tmpName).Delete(ctx)
}
