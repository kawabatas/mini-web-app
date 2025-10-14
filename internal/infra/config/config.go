package config

import (
	"os"
)

// AppConfig は環境変数を読み取りアプリ全体に渡す設定です。
type AppConfig struct {
	Port            string // HTTP ポート（未設定時は 8080）
	LogProvider     string // gcp
	LogLevel        string // -4 | 0 | 4 | 8 or debug/info/warn/error
	MaintenanceMode string // on | off

	DBDriver     string // sqlite
	SqliteSource string // local | gcs

	StorageProvider string // gcs | local(no-op)
	SqliteBucket    string // バケット名
}

func NewFromEnv() AppConfig {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return AppConfig{
		Port:            port,
		LogProvider:     os.Getenv("LOG_PROVIDER"),
		LogLevel:        os.Getenv("LOG_LEVEL"),
		MaintenanceMode: os.Getenv("MAINTENANCE_MODE"),
		DBDriver:        os.Getenv("DB_DRIVER"),
		SqliteSource:    os.Getenv("SQLITE_SOURCE"),
		StorageProvider: os.Getenv("STORAGE_PROVIDER"),
		SqliteBucket:    os.Getenv("SQLITE_BUCKET"),
	}
}

// SnapshotEnabled はスナップショット同期を有効化すべきかの判定です。
func (c AppConfig) SnapshotEnabled() bool {
	return c.StorageProvider == "gcs" && c.SqliteBucket != ""
}
