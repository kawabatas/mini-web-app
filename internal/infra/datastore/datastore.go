package datastore

import (
	"context"

	"github.com/kawabatas/mini-web-app/internal/domain/repository"
)

// DataStore is an app-facing facade for all repositories.
type DataStore interface {
	Ping(ctx context.Context) error
	Close() error
	// SetConnPool は接続プール設定を適用します。
	SetConnPool(maxOpen, maxIdle int)

	// 個別の実装
	Singers() repository.SingerRepository
}

// Config captures DB driver and DSN-like parameters.
type Config struct {
	Driver   string // e.g. "sqlite" (default)
	Source   string // extra hint for path decisions (e.g., "gcs")
	Strategy SnapshotStrategy
}

// Open selects and opens a datastore by driver.
func Open(ctx context.Context, cfg Config) (DataStore, error) {
	switch cfg.Driver {
	case "sqlite":
		return openSQLite(ctx, cfg)
	default:
		return openSQLite(ctx, cfg)
	}
}
