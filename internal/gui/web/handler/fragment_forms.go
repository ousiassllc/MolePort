package handler

import (
	"net/http"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/web/view/organisms"
)

// Forms は操作系のフラグメント（/fragments/{forward-form,forward-delete}）。モーダルの内容を返す。
// 内蔵デーモンが起動していない間は 409 + 「起動中です」トーストを返す。
type Forms struct {
	life service.LifecycleSource
	fwd  service.Forwards
}

// NewForms は Forms を返す。
func NewForms(life service.LifecycleSource, fwd service.Forwards) *Forms {
	return &Forms{life: life, fwd: fwd}
}

func (f *Forms) ready(w http.ResponseWriter, r *http.Request) bool {
	if f.life.State() == service.LifecycleReady {
		return true
	}
	problem(w, r, service.ErrNotReady, eventForwards, "")
	return false
}

// ForwardForm は GET /fragments/forward-form。
// クエリに type があれば種別切り替えの再取得として現在の入力値を引き継ぎ、なければ host を選択済みにした初期フォームを返す。
func (f *Forms) ForwardForm(w http.ResponseWriter, r *http.Request) {
	if !f.ready(w, r) {
		return
	}
	q := r.URL.Query()
	in := f.fwd.Form(q.Get("host"))
	if q.Has("type") {
		in = forwardInput(q)
	}
	render(w, r, http.StatusOK, organisms.ForwardForm(in, nil, f.fwd.HostNames()))
}

// ForwardDelete は GET /fragments/forward-delete?name=...。削除確認ダイアログを返す。
func (f *Forms) ForwardDelete(w http.ResponseWriter, r *http.Request) {
	if !f.ready(w, r) {
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	render(w, r, http.StatusOK, organisms.ForwardDeleteDialog(name))
}
