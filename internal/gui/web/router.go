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
	Life   service.LifecycleSource // 内蔵デーモンの起動状態
	Token  string                  // 起動ごとのランダムトークン（middleware.NewToken）
	Static fs.FS                   // go:embed の dist 含む FS。テストでは fstest.MapFS
}

// NewRouter は ServeMux とミドルウェアチェーンを組み立てた http.Handler を返す。
// リッスンは行わない。ミドルウェアは外側から
// Recover → SecurityHeaders → Logging → Token → HXRequired の順に適用される。
func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handler.NewPage(deps.Life, deps.Token).Index)
	mux.Handle("GET /static/{path...}", handler.NewStatic(deps.Static))

	return middleware.Chain(mux,
		middleware.Recover,
		middleware.SecurityHeaders,
		middleware.Logging,
		middleware.Token(deps.Token),
		middleware.HXRequired,
	)
}
