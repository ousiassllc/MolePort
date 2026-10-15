// Package web は GUI の Web 層。net/http のルーターとミドルウェアを組み立てる。
// Wails・トレイに依存せず、TCP/UDP のポートも開かない（NFR-34）。
// Wails の AssetServer.Handler に NewRouter の戻り値を渡す。
package web

import (
	"io/fs"
	"net/http"

	"github.com/ousiassllc/moleport/internal/core/guiconfig"
	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/service/credential"
	"github.com/ousiassllc/moleport/internal/gui/service/settings"
	"github.com/ousiassllc/moleport/internal/gui/web/handler"
	"github.com/ousiassllc/moleport/internal/gui/web/middleware"
)

// Deps は Router が必要とする依存。
type Deps struct {
	Life      service.LifecycleSource    // 内蔵デーモンの起動状態
	Dashboard service.Dashboard          // ダッシュボードの ViewModel
	Toasts    *service.NotificationQueue // 非同期通知のキュー（/fragments/toasts が取り出す）
	Forwards  service.Forwards           // 転送の追加・開始・停止・削除
	Hosts     service.Hosts              // ホストの接続・切断
	Cred      credential.Prompter        // 待機中のクレデンシャル要求（CredentialBroker）
	Settings  settings.Settings          // 設定画面・リリースページの起動
	Theme     guiconfig.Theme            // 内蔵デーモンの起動前に描画するページのテーマ（起動時に読んだ gui.theme）
	Token     string                     // 起動ごとのランダムトークン（middleware.NewToken）
	Static    fs.FS                      // go:embed の dist 含む FS。テストでは fstest.MapFS
}

// NewRouter は ServeMux とミドルウェアチェーンを組み立てた http.Handler を返す。
// リッスンは行わない。ミドルウェアは外側から
// Recover → SecurityHeaders → Logging → Token → HXRequired の順に適用される。
func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()
	frags := handler.NewFragments(deps.Life, deps.Dashboard, deps.Toasts)
	mux.HandleFunc("GET /{$}", handler.NewPage(deps.Life, deps.Dashboard, deps.Settings, deps.Theme, deps.Token).Index)
	mux.HandleFunc("GET /fragments/header", frags.Header)
	mux.HandleFunc("GET /fragments/hosts", frags.Hosts)
	mux.HandleFunc("GET /fragments/forwards", frags.Forwards)
	mux.HandleFunc("GET /fragments/status", frags.Status)
	mux.HandleFunc("GET /fragments/toasts", frags.Toasts)

	forms := handler.NewForms(deps.Life, deps.Forwards)
	mux.HandleFunc("GET /fragments/forward-form", forms.ForwardForm)
	mux.HandleFunc("GET /fragments/forward-delete", forms.ForwardDelete)
	cred := handler.NewCredential(deps.Cred)
	mux.HandleFunc("GET /fragments/credential", cred.Dialog)
	mux.HandleFunc("POST /actions/credential/{id}", cred.Resolve)
	mux.HandleFunc("POST /actions/credential/{id}/cancel", cred.Cancel)
	set := handler.NewSettings(deps.Settings)
	mux.HandleFunc("GET /fragments/settings", set.Form)
	mux.HandleFunc("POST /actions/settings", set.Save)
	mux.HandleFunc("POST /actions/update/open", set.OpenUpdate)

	fwd := handler.NewForwardActions(deps.Forwards, deps.Dashboard)
	mux.HandleFunc("POST /actions/forwards", fwd.Add)
	mux.HandleFunc("POST /actions/forwards/{name}/start", fwd.Start)
	mux.HandleFunc("POST /actions/forwards/{name}/stop", fwd.Stop)
	mux.HandleFunc("DELETE /actions/forwards/{name}", fwd.Delete)
	hosts := handler.NewHostActions(deps.Hosts, deps.Dashboard)
	mux.HandleFunc("POST /actions/hosts/{name}/connect", hosts.Connect)
	mux.HandleFunc("POST /actions/hosts/{name}/disconnect", hosts.Disconnect)
	mux.Handle("GET /static/{path...}", handler.NewStatic(deps.Static))

	return middleware.Chain(mux,
		middleware.Recover,
		middleware.SecurityHeaders,
		middleware.Logging,
		middleware.Token(deps.Token),
		middleware.HXRequired,
	)
}
