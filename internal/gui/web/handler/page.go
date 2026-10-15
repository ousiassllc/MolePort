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
	token string
}

// NewPage は Page を返す。token は <meta name="mp-token"> に埋め込む起動ごとのトークン。
func NewPage(life service.LifecycleSource, token string) *Page {
	return &Page{life: life, token: token}
}

// Index は BaseLayout + ライフサイクル状態に応じたバナーを返す。
func (p *Page) Index(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, pages.DashboardPage(layout.LayoutProps{
		Lang:   string(i18n.CurrentLang()),
		Theme:  defaultTheme,
		Accent: defaultAccent,
		Token:  p.token,
	}, p.life.State()))
}
