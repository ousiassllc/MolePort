package middleware

import (
	"log/slog"
	"net/http"
	"strings"
)

// credentialPrefix はクレデンシャルを受け取るルート。リクエストログから除外する（NFR-36）。
const credentialPrefix = "/actions/credential" //nolint:gosec // ルートのパス接頭辞であり秘密情報ではない

// Logging はリクエストを Debug レベルで記録する。メソッドとパスのみを記録し、
// クエリ・ヘッダー・本文は含めない。クレデンシャルルートは一切記録しない。
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, credentialPrefix) {
			slog.Debug("gui: request", "method", r.Method, "path", r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}
