package middleware

import (
	"net/http"
	"strings"
)

const fragmentsPrefix = "/fragments/"

// HXRequired は /fragments/ 配下に HX-Request: true を要求する。
// ブラウザからの直接アクセスによる断片表示を防ぐため、なければ 400。
func HXRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, fragmentsPrefix) && r.Header.Get("HX-Request") != "true" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}
