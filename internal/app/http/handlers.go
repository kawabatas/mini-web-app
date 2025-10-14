package apphttp

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/kawabatas/mini-web-app/internal/app/usecase"
	"github.com/kawabatas/mini-web-app/internal/infra/datastore"
)

// Register wires API endpoints onto the provided mux.
func Register(mux *http.ServeMux, ds datastore.DataStore) {
	mux.HandleFunc("GET /healthz", healthz(ds)) // DB接続も確認するため healthz

	// Singerはサンプル実装です。
	svc := usecase.NewSingerService(ds)
	mux.HandleFunc("GET /api/v1/singers", listSingers(svc))
}

func healthz(ds datastore.DataStore) http.HandlerFunc {
	type resp struct {
		Status string `json:"status"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if err := ds.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, resp{Status: "ng"})
			return
		}
		writeJSON(w, http.StatusOK, resp{Status: "ok"})
	}
}

// listSingers はユースケース層（SingerService）を利用して一覧を返します。
func listSingers(svc *usecase.SingerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit := atoiDefault(q.Get("limit"), 50)
		offset := atoiDefault(q.Get("offset"), 0)
		params := usecase.SingerListParams{
			Limit:  limit,
			Offset: offset,
		}
		result, err := svc.List(r.Context(), params)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list singers")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}
}

// 文字列を int に変換（不正時はデフォルト値）
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}
