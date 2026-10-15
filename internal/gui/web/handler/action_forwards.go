package handler

import (
	"net/http"
	"net/url"

	"github.com/a-h/templ"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/web/view/organisms"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// ForwardActions は転送のアクション（/actions/forwards...）。
// 成功時は更新後の転送パネルを本文に含め、副作用（追加・削除）はトーストを OOB で返す。
type ForwardActions struct {
	fwd  service.Forwards
	dash service.Dashboard
}

// NewForwardActions は ForwardActions を返す。
func NewForwardActions(fwd service.Forwards, dash service.Dashboard) *ForwardActions {
	return &ForwardActions{fwd: fwd, dash: dash}
}

// forwardInput はフォームの値（POST ボディ・種別切り替えの GET クエリ共通）を ForwardFormInput にする。
func forwardInput(v url.Values) service.ForwardFormInput {
	return service.ForwardFormInput{
		Host:           v.Get("host"),
		Type:           v.Get("type"),
		Name:           v.Get("name"),
		LocalPort:      v.Get("local_port"),
		RemoteHost:     v.Get("remote_host"),
		RemotePort:     v.Get("remote_port"),
		RemoteBindAddr: v.Get("remote_bind_addr"),
		AutoConnect:    v.Get("auto_connect") == "on",
	}
}

func (a *ForwardActions) panel() templ.Component {
	return organisms.ForwardPanel(a.dash.Forwards(), true)
}

// Add は POST /actions/forwards。検証エラーは 422 でフォームを再描画する。
func (a *ForwardActions) Add(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	in := forwardInput(r.PostForm)
	name, errs, err := a.fwd.Add(in)
	if err != nil {
		problem(w, r, err, eventForwards, "")
		return
	}
	if len(errs) > 0 {
		invalid(w, r, "#modal-root", organisms.ForwardForm(in, errs, a.fwd.HostNames()))
		return
	}
	setTrigger(w, eventCloseModal)
	toast := toastOOB(service.ToastSuccess, i18n.T("gui.forward.toast.added", map[string]any{"Name": name}))
	render(w, r, http.StatusOK, templ.Join(a.panel(), toast))
}

// Start は POST /actions/forwards/{name}/start。認証待ちでブロックせず、更新後のパネルを即座に返す。
func (a *ForwardActions) Start(w http.ResponseWriter, r *http.Request) {
	a.transition(w, r, a.fwd.Start)
}

// Stop は POST /actions/forwards/{name}/stop。
func (a *ForwardActions) Stop(w http.ResponseWriter, r *http.Request) {
	a.transition(w, r, a.fwd.Stop)
}

func (a *ForwardActions) transition(w http.ResponseWriter, r *http.Request, op func(string) error) {
	name := r.PathValue("name")
	if err := op(name); err != nil {
		problem(w, r, err, eventForwards, i18n.T("gui.forward.toast.conflict", map[string]any{"Name": name}))
		return
	}
	render(w, r, http.StatusOK, a.panel())
}

// Delete は DELETE /actions/forwards/{name}。停止して削除し、確認ダイアログを閉じる。
func (a *ForwardActions) Delete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := a.fwd.Delete(name); err != nil {
		problem(w, r, err, eventForwards, "", eventCloseModal)
		return
	}
	setTrigger(w, eventCloseModal)
	toast := toastOOB(service.ToastSuccess, i18n.T("gui.forward.toast.deleted", map[string]any{"Name": name}))
	render(w, r, http.StatusOK, templ.Join(a.panel(), toast))
}
