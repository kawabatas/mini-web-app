package datastore

import "context"

// SnapshotStrategy はドライバに依存したスナップショットの開始/終了フックを表します。
// - SQLite では VACUUM INTO + オブジェクトストレージへの二相アップロードを実装
// - 他ドライバでは何もしない実装を与えることができます。
type SnapshotStrategy interface {
	OnStartup(ctx context.Context, dbPath string) error
	OnShutdown(ctx context.Context, dbPath string) error
}

// NoopSnapshotStrategy は何もしない実装です。
type NoopSnapshotStrategy struct{}

func (NoopSnapshotStrategy) OnStartup(ctx context.Context, dbPath string) error  { return nil }
func (NoopSnapshotStrategy) OnShutdown(ctx context.Context, dbPath string) error { return nil }
