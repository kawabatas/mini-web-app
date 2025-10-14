package httpx

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// CachingFileServer serves static files with sane cache headers for SPA.
func CachingFileServer(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath := r.URL.Path
		ext := strings.ToLower(filepath.Ext(requestPath))

		localPath := filepath.Join(dir, filepath.Clean(requestPath))

		if fi, err := os.Stat(localPath); err == nil && !fi.IsDir() {
			switch {
			case ext == ".html":
				// HTML は 毎回再検証 し、デプロイ後すぐ新しい HTML を配る
				// ネットワーク障害時などにキャッシュが使われることを許可する
				// w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
			case strings.HasPrefix(requestPath, "/assets/") ||
				// ハッシュ付きアセットは長期キャッシュ
				ext == ".js" || ext == ".css" || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".svg" || ext == ".webp":
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			default:
				// その他は1時間
				w.Header().Set("Cache-Control", "public, max-age=3600")
			}
			http.ServeFile(w, r, localPath)
			return
		}

		// Not a file: for SPA routes serve index.html without cache
		if ext == "" || requestPath == "/" {
			w.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		http.NotFound(w, r)
	})
}
