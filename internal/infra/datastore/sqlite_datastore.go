package datastore

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/kawabatas/mini-web-app/internal/domain/repository"
	sqlitedriver "github.com/kawabatas/mini-web-app/internal/infra/datastore/sqlite"
)

type sqliteStore struct {
	db       *sql.DB
	dbPath   string
	strategy SnapshotStrategy

	singer repository.SingerRepository
}

func (s *sqliteStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *sqliteStore) Close(ctx context.Context) error {
	// 終了時のスナップショットは Strategy に委譲
	if s.strategy != nil {
		if err := s.strategy.OnShutdown(ctx, s.dbPath); err != nil {
			slog.ErrorContext(ctx, "snapshot shutdown failed", slog.Any("error", err))
		}
	}
	return s.db.Close()
}

// SetConnPool は SQLite の接続プール設定を適用します。
// - maxOpen: 同時に開ける最大接続数
// - maxIdle: アイドル接続の最大数
func (s *sqliteStore) SetConnPool(maxOpen, maxIdle int) {
	if maxOpen > 0 {
		s.db.SetMaxOpenConns(maxOpen)
	}
	if maxIdle >= 0 {
		s.db.SetMaxIdleConns(maxIdle)
	}
}

// internal interface for optional backup capability on strategy
type backupCapable interface {
	OnBackup(ctx context.Context, dbPath string) error
}

// Backup creates a consistent snapshot of the SQLite DB without closing connections.
func (s *sqliteStore) Backup(ctx context.Context) error {
	if s.strategy != nil {
		if b, ok := any(s.strategy).(backupCapable); ok {
			return b.OnBackup(ctx, s.dbPath)
		}
	}
	// Default: local snapshot into ./tmp/backups
	dir := filepath.Join("./tmp", "backups")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	out := filepath.Join(dir, fmt.Sprintf("app-snapshot-%s.sqlite", time.Now().UTC().Format("20060102-150405")))
	if err := sqlitedriver.SnapshotTo(ctx, s.dbPath, out); err != nil {
		return err
	}
	slog.InfoContext(ctx, "local snapshot created", slog.String("path", out))
	return nil
}

func openSQLite(ctx context.Context, cfg Config) (DataStore, error) {
	dbPath := sqlitedriver.Path(cfg.Source)
	// 起動時のスナップショットは Strategy に委譲
	if cfg.Strategy != nil {
		if err := cfg.Strategy.OnStartup(ctx, dbPath); err != nil {
			return nil, err
		}
	}
	db, err := sqlitedriver.OpenAndInit(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	return &sqliteStore{
		db:       db,
		dbPath:   dbPath,
		strategy: cfg.Strategy,
		singer:   sqlitedriver.NewSingerRepo(db),
	}, nil
}

func (s *sqliteStore) Singers() repository.SingerRepository { return s.singer }
