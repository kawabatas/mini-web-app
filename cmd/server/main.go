package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apphttp "github.com/kawabatas/mini-web-app/internal/app/http"
	"github.com/kawabatas/mini-web-app/internal/httpx"
	"github.com/kawabatas/mini-web-app/internal/infra/config"
	"github.com/kawabatas/mini-web-app/internal/infra/datastore"
	sqlitestrat "github.com/kawabatas/mini-web-app/internal/infra/datastore/sqlite"
	"github.com/kawabatas/mini-web-app/internal/infra/platform/logger"
	storageif "github.com/kawabatas/mini-web-app/internal/infra/storage"
	gcsstore "github.com/kawabatas/mini-web-app/internal/infra/storage/gcs"
	localstore "github.com/kawabatas/mini-web-app/internal/infra/storage/local"
)

func main() {
	cfg := config.NewFromEnv()
	lvl := logger.ParseLevel(cfg.LogLevel)
	slog.SetDefault(logger.New(cfg.LogProvider, lvl))

	ctx := context.Background()

	var objStore storageif.ObjectStore = localstore.Noop{}
	if cfg.StorageProvider == "gcs" && cfg.SqliteBucket != "" {
		objStore = &gcsstore.Adapter{}
	}

	// スナップショット戦略を選択
	var strat datastore.SnapshotStrategy = datastore.NoopSnapshotStrategy{}
	if cfg.DBDriver == "" || cfg.DBDriver == "sqlite" {
		if cfg.SnapshotEnabled() {
			strat = sqlitestrat.GCSSnapshotStrategy{ObjectStore: objStore, Bucket: cfg.SqliteBucket}
		} else {
			strat = sqlitestrat.LocalSnapshotStrategy{OutputDir: ""}
		}
	}
	ds, err := datastore.Open(ctx, datastore.Config{Driver: cfg.DBDriver, Source: cfg.SqliteSource, Strategy: strat})
	if err != nil {
		log.Fatalf("datastore open error: %v", err)
	}
	// 接続プール設定: 最大接続・アイドルともに 10
	ds.SetConnPool(10, 10)
	// defer ds.Close() --- シャットダウン時に呼び出し ---

	mux := http.NewServeMux()
	apphttp.Register(mux, ds)
	// Static (serve built assets)
	mux.Handle("/", httpx.CachingFileServer("./frontend/dist"))

	handler := httpx.LoggingMiddleware(httpx.MaintenanceMiddleware(httpx.RecoverMiddleware(mux)))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           http.TimeoutHandler(handler, 5*time.Second, "timeout"),
		ReadHeaderTimeout: 500 * time.Millisecond,
		ReadTimeout:       500 * time.Millisecond,
		IdleTimeout:       time.Second,
	}

	// 定期バックアップ（VACUUM INTO の負荷を避けるためデフォルトoff）
	if (cfg.DBDriver == "" || cfg.DBDriver == "sqlite") && cfg.PeriodicBackupEnabled() {
		go func() {
			ticker := time.NewTicker(time.Duration(cfg.PeriodicBackupIntervalMinutes()) * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				ctxSnap, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				slog.InfoContext(ctxSnap, "periodic snapshot start")
				if err := ds.Backup(ctxSnap); err != nil {
					slog.ErrorContext(ctxSnap, "periodic snapshot failed", slog.Any("error", err))
				} else {
					slog.InfoContext(ctxSnap, "periodic snapshot complete")
				}
				cancel()
			}
		}()
	}

	go func() {
		slog.InfoContext(ctx, "server starting", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
		slog.InfoContext(ctx, "server stopped accepting new conns")
	}()

	// シャットダウン待受け
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	// Cloud Run の場合、SIGTERM から約 10 秒後に強制終了される
	// https://cloud.google.com/blog/ja/products/application-development/graceful-shutdowns-cloud-run-deep-dive
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	if err := ds.Close(ctxShutdown); err != nil {
		log.Fatalf("datastore close error: %v", err)
	}

	slog.InfoContext(ctxShutdown, "graceful shutdown complete")
}
