package datastore

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/kawabatas/mini-web-app/internal/domain/repository"
	sqlitedriver "github.com/kawabatas/mini-web-app/internal/infra/datastore/sqlite"
)

type sqliteStore struct {
	ctx      context.Context
	db       *sql.DB
	dbPath   string
	strategy SnapshotStrategy

	singer repository.SingerRepository
}

func (s *sqliteStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *sqliteStore) Close() error {
	// 終了時のスナップショットは Strategy に委譲
	if s.strategy != nil {
		if err := s.strategy.OnShutdown(s.ctx, s.dbPath); err != nil {
			slog.ErrorContext(s.ctx, "snapshot shutdown failed", slog.Any("error", err))
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
		ctx:      ctx,
		db:       db,
		dbPath:   dbPath,
		strategy: cfg.Strategy,
		singer:   sqlitedriver.NewSingerRepo(db),
	}, nil
}

func (s *sqliteStore) Singers() repository.SingerRepository { return s.singer }
