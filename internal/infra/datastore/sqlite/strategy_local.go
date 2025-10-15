package sqlite

import (
	"context"
	"os"
	"path/filepath"

	"github.com/kawabatas/mini-web-app/internal/util/clock"
)

// LocalSnapshotStrategy creates a local snapshot on shutdown into OutputDir.
type LocalSnapshotStrategy struct {
	OutputDir string
}

func (LocalSnapshotStrategy) OnStartup(ctx context.Context, dbPath string) error { return nil }

func (s LocalSnapshotStrategy) OnShutdown(ctx context.Context, dbPath string) error {
	dir := s.OutputDir
	if dir == "" {
		dir = filepath.Join("./tmp", "backups")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	snap := filepath.Join(dir, "app-snapshot-"+clock.NowUTCFormatted("20060102-150405")+".sqlite")
	return SnapshotTo(ctx, dbPath, snap)
}
