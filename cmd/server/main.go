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

	// スナップショット戦略（SQLite のみ）を選択
	var strat datastore.SnapshotStrategy = datastore.NoopSnapshotStrategy{}
	if cfg.DBDriver == "" || cfg.DBDriver == "sqlite" {
		if cfg.SnapshotEnabled() {
			strat = sqlitestrat.GCSSnapshotStrategy{ObjectStore: objStore, Bucket: cfg.SqliteBucket}
		}
	}
	ds, err := datastore.Open(ctx, datastore.Config{Driver: cfg.DBDriver, Source: cfg.SqliteSource, Strategy: strat})
	if err != nil {
		log.Fatalf("datastore open error: %v", err)
	}
	// 接続プール設定: 最大接続・アイドルともに 10
	ds.SetConnPool(10, 10)
	defer ds.Close()

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

	go func() {
		slog.Info("server starting", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
		slog.Info("server stopped accepting new conns")
	}()

	// シャットダウン待受け
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	slog.Info("graceful shutdown complete")
}
