package handler

import (
	"net/http"

	"github.com/ousiassllc/moleport/internal/core/guiconfig"
	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/service/settings"
	"github.com/ousiassllc/moleport/internal/gui/web/view/layout"
	"github.com/ousiassllc/moleport/internal/gui/web/view/pages"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// Page は GET /（フルページ）を返すハンドラー。
type Page struct {
	life    service.LifecycleSource
	dash    service.Dashboard
	set     settings.Settings
	initial guiconfig.Theme
	token   string
}

// NewPage は Page を返す。initial は内蔵デーモンの起動前（設定を読めない間）に使うテーマ、
// token は <meta name="mp-token"> に埋め込む起動ごとのトークン。
func NewPage(life service.LifecycleSource, dash service.Dashboard, set settings.Settings, initial guiconfig.Theme, token string) *Page {
	return &Page{life: life, dash: dash, set: set, initial: initial.Normalized(), token: token}
}

// Index は BaseLayout + バナー + ダッシュボード各領域（インライン展開）を返す。
// <html> の data-theme / data-accent は config.yaml の gui.theme の保存値。
func (p *Page) Index(w http.ResponseWriter, r *http.Request) {
	state := p.life.State()
	theme := p.initial
	if v, err := p.set.View(); err == nil {
		theme = guiconfig.Theme{Base: v.ThemeBase, Accent: v.ThemeAccent}
	}
	props := pages.DashboardProps{
		Layout: layout.LayoutProps{
			Lang:   string(i18n.CurrentLang()),
			Theme:  theme.Base,
			Accent: theme.Accent,
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
