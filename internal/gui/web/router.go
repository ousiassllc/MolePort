// Package web は GUI の Web 層。net/http のルーターとミドルウェアを組み立てる。
// Wails・トレイに依存せず、TCP/UDP のポートも開かない（NFR-34）。
// Wails の AssetServer.Handler に NewRouter の戻り値を渡す。
package web

import (
	"io/fs"
	"net/http"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/web/handler"
	"github.com/ousiassllc/moleport/internal/gui/web/middleware"
)

// Deps は Router が必要とする依存。
type Deps struct {
	Life      service.LifecycleSource    // 内蔵デーモンの起動状態
	Dashboard service.Dashboard          // ダッシュボードの ViewModel
	Toasts    *service.NotificationQueue // 非同期通知のキュー（/fragments/toasts が取り出す）
	Token     string                     // 起動ごとのランダムトークン（middleware.NewToken）
	Static    fs.FS                      // go:embed の dist 含む FS。テストでは fstest.MapFS
}

// NewRouter は ServeMux とミドルウェアチェーンを組み立てた http.Handler を返す。
// リッスンは行わない。ミドルウェアは外側から
// Recover → SecurityHeaders → Logging → Token → HXRequired の順に適用される。
func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()
	frags := handler.NewFragments(deps.Life, deps.Dashboard, deps.Toasts)
	mux.HandleFunc("GET /{$}", handler.NewPage(deps.Life, deps.Dashboard, deps.Token).Index)
	mux.HandleFunc("GET /fragments/header", frags.Header)
	mux.HandleFunc("GET /fragments/hosts", frags.Hosts)
	mux.HandleFunc("GET /fragments/forwards", frags.Forwards)
	mux.HandleFunc("GET /fragments/status", frags.Status)
	mux.HandleFunc("GET /fragments/toasts", frags.Toasts)
	mux.Handle("GET /static/{path...}", handler.NewStatic(deps.Static))

	return middleware.Chain(mux,
		middleware.Recover,
		middleware.SecurityHeaders,
		middleware.Logging,
		middleware.Token(deps.Token),
		middleware.HXRequired,
	)
}
