package middleware

import "net/http"

// Middleware は http.Handler をラップする関数。
type Middleware func(http.Handler) http.Handler

// Chain は mws を外側から内側の順に h へ適用する。
// Chain(h, A, B) のリクエストは A → B → h の順に通過する。
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
