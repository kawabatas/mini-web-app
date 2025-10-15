package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const FileName = "app.sqlite"

// Path decides DB file path for given source.
// - "gcs": use /tmp for Cloud Run ephemeral FS
// - otherwise: local ./tmp
func Path(source string) string {
	if source == "gcs" {
		return filepath.Join("/tmp", FileName)
	}
	_ = os.MkdirAll("./tmp", 0755)
	return filepath.Join("./tmp", FileName)
}

// PRAGMAの意味:
//
//	journal_mode=WAL: 同時実行性向上のためWALモードを有効化
//	synchronous=NORMAL: 性能と耐障害性のバランスを取る
//	busy_timeout: ロック競合時の自動リトライ待機時間（ms）
const busyTimeoutMs = 2000 // HTTPリクエストタイムアウト(2s)に合わせる

func dsnWithPragma(path string) string {
	return fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(%d)", path, busyTimeoutMs)
}

func OpenAndInit(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsnWithPragma(path))
	if err != nil {
		return nil, err
	}
	if err := initSchemaAndSeed(ctx, db); err != nil {
		return nil, err
	}
	return db, nil
}

func initSchemaAndSeed(ctx context.Context, db *sql.DB) error {
	// スキーマ初期化
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS singers (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  genre TEXT NOT NULL,
  debut_year INTEGER NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	// データが空なら初期データ投入
	var cnt int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM singers`).Scan(&cnt); err == nil && cnt == 0 {
		_, _ = db.ExecContext(ctx, `INSERT INTO singers(name, genre, debut_year) VALUES
			('Taylor Swift','Pop',2006),
			('Ed Sheeran','Pop',2011),
			('Adele','Soul',2008)
		`)
	}
	return nil
}

// SnapshotTo は VACUUM INTO を用いて、SQLite DB の一貫したスナップショットを作成します。
//
// 注記:
//   - 可能であれば SQLite の Online Backup API（sqlite3_backup_*）を利用したいところですが、
//     本プロジェクトで使用している pure Go のドライバ（modernc.org/sqlite）では同 API が
//     直接は提供されていないため、VACUUM INTO によるスナップショット方式を採用しています。
//     VACUUM INTO は実行中に他の書き込み読み取りをブロックします。
//
// 実装メモ:
// - busy_timeout を付けた別接続で開くことで、即時の SQLITE_BUSY を避けます。
// - 書き込みの競合などで BUSY の場合は、短いバックオフ付きで数回リトライします。
// - outPath は信頼できるパスのみを渡すこと（VACUUM INTO はパラメータ化できないためSQLインジェクション注意）。
func SnapshotTo(ctx context.Context, dbPath, outPath string) error {
	const (
		maxRetries    = 3
		baseBackoffMs = 200
	)

	db, err := sql.Open("sqlite", dsnWithPragma(dbPath))
	if err != nil {
		return err
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			slog.WarnContext(ctx, "snapshot: db close error", slog.Any("error", cerr))
		}
	}()

	// WALモードでもVACUUM INTOは一貫したコピーを生成できる
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		// WALファイル肥大化対策: チェックポイントでWALをtruncate
		_, _ = db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE);")
		// outPathは信頼できるパスのみを渡すこと（SQLインジェクション注意）
		vacuumSQL := fmt.Sprintf(`VACUUM INTO '%s';`, outPath)
		_, err := db.ExecContext(ctx, vacuumSQL)
		if err == nil {
			slog.InfoContext(ctx, "snapshot: success", slog.Int("attempt", i+1))
			return nil
		}
		lastErr = err
		if isBusyErr(err) {
			backoff := baseBackoffMs * (i + 1)
			slog.WarnContext(ctx, "snapshot: busy, retrying", slog.Int("attempt", i+1), slog.Int("sleep_ms", backoff), slog.Any("error", err))
			select {
			case <-timeAfter(ctx, backoff):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			slog.ErrorContext(ctx, "snapshot: failed", slog.Int("attempt", i+1), slog.Any("error", err))
			return err
		}
	}
	slog.ErrorContext(ctx, "snapshot: all retries failed", slog.Any("error", lastErr))
	return lastErr
}

// isBusyErr は SQLITE_BUSY（"database is locked"）系エラーを判定します。
func isBusyErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "SQLITE_BUSY") || strings.Contains(s, "database is locked")
}

// timeAfter は ms ミリ秒待機するか、ctx が終了したら返します。
func timeAfter(ctx context.Context, ms int) <-chan struct{} {
	ch := make(chan struct{})
	t := time.NewTimer(time.Duration(ms) * time.Millisecond)
	go func() {
		defer close(ch)
		defer t.Stop()
		select {
		case <-t.C:
		case <-ctx.Done():
		}
	}()
	return ch
}
