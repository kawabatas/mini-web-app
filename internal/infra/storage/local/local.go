package local

import "context"

// Noop implements ObjectStore with no-ops for local usage.
type Noop struct{}

func (Noop) DownloadIfNeeded(ctx context.Context, bucket, object, dest string) error { return nil }
func (Noop) UploadTwoPhaseWithBackup(ctx context.Context, bucket, currentObject, backupObject, localPath string) error {
	return nil
}
