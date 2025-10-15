package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	sqlitedriver "github.com/kawabatas/mini-web-app/internal/infra/datastore/sqlite"
	"github.com/kawabatas/mini-web-app/internal/util/clock"
	_ "modernc.org/sqlite"
)

const (
	dbPath       = "./tmp/sim_app.sqlite"
	backupPath   = "./tmp/sim_app-backup.sqlite"
	writers      = 4  // 並列ライター数
	rowsPerTx    = 50 // 1トランザクションあたりの行数
	testDuration = 10 * time.Second
)

type backupResult struct {
	at          time.Time
	duration    time.Duration
	mainCount   int64
	backupCount int64
	integrity   string
	err         error
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
        PRAGMA foreign_keys=ON;
        CREATE TABLE IF NOT EXISTS events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            writer_id INTEGER NOT NULL,
            seq INTEGER NOT NULL,
            payload TEXT NOT NULL,
            created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
    `)
	return err
}

func writer(ctx context.Context, id int, db *sql.DB, wg *sync.WaitGroup) {
	defer wg.Done()

	seq := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			log.Printf("[writer %d] begin err: %v", id, err)
			time.Sleep(20 * time.Millisecond)
			continue
		}

		stmt, err := tx.PrepareContext(ctx, `INSERT INTO events (writer_id, seq, payload) VALUES (?, ?, ?)`)
		if err != nil {
			_ = tx.Rollback()
			log.Printf("[writer %d] prep err: %v", id, err)
			continue
		}

		for i := 0; i < rowsPerTx; i++ {
			seq++
			if _, err := stmt.ExecContext(ctx, id, seq, fmt.Sprintf("w%d-%d", id, seq)); err != nil {
				// busy の場合は軽く待ってリトライ（同じ i を再試行）
				if isBusyErr(err) {
					seq-- // この行は未挿入なので seq を戻す
					i--
					time.Sleep(time.Duration(10+rand.Intn(20)) * time.Millisecond)
					continue
				}
				log.Printf("[writer %d] exec err: %v", id, err)
			}
			// ほんの少しゆらぎを入れてロック競合を発生させやすくする
			time.Sleep(time.Duration(rand.Intn(3)) * time.Millisecond)
		}
		_ = stmt.Close()

		if err := tx.Commit(); err != nil {
			if isBusyErr(err) {
				// 軽く待ってやり直し
				time.Sleep(20 * time.Millisecond)
				continue
			}
			log.Printf("[writer %d] commit err: %v", id, err)
		}

		// 書き込みペース
		time.Sleep(time.Duration(rand.Intn(15)) * time.Millisecond)
	}
}

func isBusyErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "SQLITE_BUSY") || strings.Contains(s, "database is locked")
}

func backupOnce(ctx context.Context, dbPath, path string) error {
	// infra/datastore/sqlite の SnapshotTo を利用
	// - VACUUM INTO は出力先が既に存在すると失敗するため、.tmp に出力してから置換する
	// - 一時的なロック競合を想定して、軽いリトライを入れる
	tmp := path + ".tmp"
	_ = os.Remove(tmp)
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	var lastErr error
	for i := 0; i < 3; i++ {
		if err := sqlitedriver.SnapshotTo(ctx, dbPath, tmp); err != nil {
			lastErr = err
			if isBusyErr(err) {
				time.Sleep(time.Duration(50*(i+1)) * time.Millisecond)
				continue
			}
			return err
		}
		// 既存のスナップショットを置き換える
		_ = os.Remove(path)
		if err := os.Rename(tmp, path); err != nil {
			lastErr = err
			// 失敗した場合は次回のために tmp を消してから終了
			_ = os.Remove(tmp)
			return err
		}
		return nil
	}
	return lastErr
}

func countRows(db *sql.DB) (int64, error) {
	var n int64
	err := db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&n)
	return n, err
}

func integrityCheck(path string) (string, error) {
	// 別コネクションで対象 DB をオープンして integrity_check
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return "", err
	}
	defer db.Close()

	var res string
	if err := db.QueryRow(`PRAGMA integrity_check;`).Scan(&res); err != nil {
		return "", err
	}
	return res, nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)

	db, err := sqlitedriver.OpenAndInit(context.Background(), dbPath)
	must(err)
	defer db.Close()
	// 接続プール設定（サーバと同様に 10 を使用）
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	must(initSchema(db))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go writer(ctx, i+1, db, &wg)
	}

	// バックアップループ
	var results []backupResult
	done := time.After(testDuration)
loop:
	for {
		select {
		case <-done:
			break loop
		case <-time.After(2 * time.Second):
			start := clock.Now()
			r := backupResult{at: clock.Now()}
			if err := backupOnce(ctx, dbPath, backupPath); err != nil {
				r.err = err
				log.Printf("[backup] ERR: %v", err)
			} else {
				r.mainCount, _ = countRows(db)
				// バックアップ側の件数を確認
				if bkDB, err := sql.Open("sqlite", backupPath); err == nil {
					defer bkDB.Close()
					r.backupCount, _ = countRows(bkDB)
				} else {
					r.err = fmt.Errorf("open backup: %w", err)
				}
				r.integrity, _ = integrityCheck(backupPath)
				r.duration = time.Since(start)
				log.Printf("[backup] OK in %v  main=%d  backup=%d  integrity=%s",
					r.duration, r.mainCount, r.backupCount, r.integrity)
			}
			results = append(results, r)
		}
	}

	// 終了前に最終バックアップ
	log.Println("[backup] final...")
	must(backupOnce(ctx, dbPath, backupPath))
	ic, _ := integrityCheck(backupPath)
	mainCount, _ := countRows(db)
	bkDB, _ := sql.Open("sqlite", backupPath)
	defer func() {
		if bkDB != nil {
			bkDB.Close()
		}
	}()
	bkCount, _ := countRows(bkDB)

	log.Printf("[final] main=%d backup=%d integrity=%s", mainCount, bkCount, ic)

	stop()
	// writers 終了
	wg.Wait()

	// 検証: バックアップは一貫性があり、件数は単調増加、かつ mainCount を超えない
	passed := true
	var violations []string
	var prev int64 = -1
	for i, r := range results {
		if r.err != nil {
			passed = false
			violations = append(violations, fmt.Sprintf("%d: backup error: %v", i, r.err))
			continue
		}
		if r.integrity != "ok" {
			passed = false
			violations = append(violations, fmt.Sprintf("%d: integrity=%s", i, r.integrity))
		}
		if prev >= 0 && r.backupCount < prev {
			passed = false
			violations = append(violations, fmt.Sprintf("%d: backupCount %d < prev %d", i, r.backupCount, prev))
		}
		if r.backupCount > r.mainCount {
			passed = false
			violations = append(violations, fmt.Sprintf("%d: backupCount %d > mainCount %d", i, r.backupCount, r.mainCount))
		}
		prev = r.backupCount
	}

	if ic != "ok" {
		passed = false
		violations = append(violations, fmt.Sprintf("final integrity=%s", ic))
	}
	if bkCount > mainCount {
		passed = false
		violations = append(violations, fmt.Sprintf("final backupCount %d > mainCount %d", bkCount, mainCount))
	}

	if passed {
		log.Println("RESULT: PASS (backup during concurrent writes is consistent)")
	} else {
		log.Println("RESULT: FAIL")
		for _, v := range violations {
			log.Printf(" - %s", v)
		}
	}

	log.Println("done.")
}
