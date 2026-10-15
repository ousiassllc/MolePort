package middleware

import (
	"log/slog"
	"net/http"
)

// Recover はハンドラーの panic を捕捉して 500 を返す。
// レスポンス本文にスタックトレース等の内部情報は含めない。
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			slog.Error("gui: handler panic", "method", r.Method, "path", r.URL.Path, "panic", rec)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}
