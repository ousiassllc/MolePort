package middleware

import (
	"net/http"
	"strings"
)

// ContentSecurityPolicy は全レスポンスに付与する CSP（NFR-35）。
// インラインスクリプト・外部リソースの読み込みを禁止する。
const ContentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'"

const staticPrefix = "/static/"

// SecurityHeaders は全レスポンスに CSP・X-Content-Type-Options・Cache-Control を付与する。
// Cache-Control は /static/ 配下が no-cache（更新時に再検証）、それ以外（HTML）が no-store。
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", ContentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		if strings.HasPrefix(r.URL.Path, staticPrefix) {
			h.Set("Cache-Control", "no-cache")
		} else {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
