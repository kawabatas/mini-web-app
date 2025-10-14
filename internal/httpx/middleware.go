package httpx

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: 200}
		// - X-Request-Id
		reqID := r.Header.Get("X-Request-Id")
		// リクエストIDをContextへ紐付けて以降の処理で参照可能にする
		// TODO: OpenTelemetry 組み込む
		ctx := WithRequestID(r.Context(), reqID)
		next.ServeHTTP(rw, r.WithContext(ctx))
		dur := time.Since(start)
		attrs := []any{
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.status),
			slog.Int("duration_ms", int(dur.Milliseconds())),
		}
		if rid := RequestIDFromCtx(r.Context()); rid != "" {
			attrs = append(attrs, slog.String("request_id", rid))
		} else if reqID != "" { // 互換: ヘッダから直接
			attrs = append(attrs, slog.String("request_id", reqID))
		}
		// 開発環境でのみログ出力
		slog.DebugContext(r.Context(), fmt.Sprintf("%s %d %s", r.Method, rw.status, r.URL.Path), attrs...)
	})
}

func MaintenanceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 静的ファイル or favicon は除外
		if strings.HasPrefix(r.URL.Path, "/assets/") ||
			strings.HasSuffix(r.URL.Path, ".js") ||
			strings.HasSuffix(r.URL.Path, ".css") ||
			strings.HasSuffix(r.URL.Path, ".ico") ||
			strings.HasSuffix(r.URL.Path, ".png") ||
			strings.HasPrefix(r.URL.Path, "/favicon") {
			next.ServeHTTP(w, r)
			return
		}
		if os.Getenv("MAINTENANCE_MODE") == "on" {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "maintenance")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic",
					slog.Any("error", rec),
					slog.String("path", r.URL.Path),
					slog.String("method", r.Method),
					slog.String("stack", string(debug.Stack())),
				)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
