package handler

import (
	"net/http"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/web/view/layout"
	"github.com/ousiassllc/moleport/internal/gui/web/view/pages"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// テーマ設定の保存値を扱う Settings サービスの導入までの既定値（config の gui.theme 既定値と同じ）。
const (
	defaultTheme  = "system"
	defaultAccent = "violet"
)

// Page は GET /（フルページ）を返すハンドラー。
type Page struct {
	life  service.LifecycleSource
	dash  service.Dashboard
	token string
}

// NewPage は Page を返す。token は <meta name="mp-token"> に埋め込む起動ごとのトークン。
func NewPage(life service.LifecycleSource, dash service.Dashboard, token string) *Page {
	return &Page{life: life, dash: dash, token: token}
}

// Index は BaseLayout + バナー + ダッシュボード各領域（インライン展開）を返す。
func (p *Page) Index(w http.ResponseWriter, r *http.Request) {
	state := p.life.State()
	props := pages.DashboardProps{
		Layout: layout.LayoutProps{
			Lang:   string(i18n.CurrentLang()),
			Theme:  defaultTheme,
			Accent: defaultAccent,
			Token:  p.token,
		},
		State: state,
	}
	if state == service.LifecycleReady {
		props.Header = p.dash.Header()
		props.Hosts = p.dash.Hosts()
		props.Forwards = p.dash.Forwards()
		props.Status = p.dash.Status()
	}
	render(w, r, http.StatusOK, pages.DashboardPage(props))
}
