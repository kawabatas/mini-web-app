package logger

import (
	"log/slog"
	"strings"

	gcplogger "github.com/kawabatas/mini-web-app/internal/infra/platform/gcp/logger"
)

func New(provider string, level slog.Level) *slog.Logger {
	switch strings.ToLower(provider) {
	case "gcp":
		return gcplogger.New(level)
	default:
		return gcplogger.New(level)
	}
}

func ParseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "-4", "debug":
		return slog.LevelDebug
	case "0", "info":
		return slog.LevelInfo
	case "4", "warn":
		return slog.LevelWarn
	case "8", "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
