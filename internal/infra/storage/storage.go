package storage

import "context"

// ObjectStore abstracts a minimal object-storage API used for DB snapshots.
type ObjectStore interface {
	DownloadIfNeeded(ctx context.Context, bucket, object, dest string) error
	// UploadIfNewer(ctx context.Context, bucket, object, localPath string) error
	// UploadTwoPhaseWithBackup uploads localPath to a tmp object, atomically copies to current,
	// also writes a versioned backup object, then removes the tmp.
	UploadTwoPhaseWithBackup(ctx context.Context, bucket, currentObject, backupObject, localPath string) error
}
