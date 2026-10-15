package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/web/view/organisms"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// htmx のイベント名（app.ts が発火・購読する mp:* と同じ規約）。
const (
	eventCloseModal = "mp:close-modal"
	eventHosts      = "mp:" + string(service.TopicHosts)
	eventForwards   = "mp:" + string(service.TopicForwards)
	eventCredential = "mp:" + string(service.TopicCredential)
)

// setTrigger は HX-Trigger レスポンスヘッダーにイベント（値なし）を設定する。
func setTrigger(w http.ResponseWriter, events ...string) {
	m := make(map[string]string, len(events))
	for _, e := range events {
		m[e] = ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return // string のマップは常にエンコードできる
	}
	w.Header().Set("HX-Trigger", string(b))
}

// toastOOB は 1 件のトーストを #toast-stack へ追記する OOB コンポーネントにする。
func toastOOB(kind service.ToastKind, message string) templ.Component {
	return organisms.Toasts([]service.ToastView{{Kind: kind, Message: message}})
}

// problem は操作の同期的な失敗を HTTP 応答にする。
//   - core.NotFoundError: 404 + エラートースト。
//   - ErrStateConflict / ErrNotReady: 409 + 情報トースト（conflictMsg。ErrNotReady は起動中の案内）。
//   - それ以外: 500（内部情報を含めない。htmx:responseError が汎用エラートーストを出す）。
//
// 404 / 409 は本文を OOB トーストのみにし、HX-Reswap: none でメイン領域を変更させない。
// refresh は現状態の再取得を促すイベント（mp:hosts / mp:forwards）で、extra は同時に発火するイベント。
func problem(w http.ResponseWriter, r *http.Request, err error, refresh string, conflictMsg string, extra ...string) {
	var notFound *core.NotFoundError
	switch {
	case errors.As(err, &notFound):
		reswapNone(w, refresh, extra...)
		render(w, r, http.StatusNotFound, toastOOB(service.ToastError, service.LocalizeError(err)))
	case errors.Is(err, service.ErrStateConflict):
		reswapNone(w, refresh, extra...)
		render(w, r, http.StatusConflict, toastOOB(service.ToastInfo, conflictMsg))
	case errors.Is(err, service.ErrNotReady):
		reswapNone(w, refresh, extra...)
		render(w, r, http.StatusConflict, toastOOB(service.ToastInfo, i18n.T("gui.error.not_ready")))
	default:
		slog.Error("gui: action failed", "path", r.URL.Path, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func reswapNone(w http.ResponseWriter, refresh string, extra ...string) {
	w.Header().Set("HX-Reswap", "none")
	setTrigger(w, append([]string{refresh}, extra...)...)
}

// invalid は検証エラーのフォーム再描画を 422 で返す。htmx の既定ターゲットではなく
// モーダル（target）内へ差し替える（HX-Retarget / HX-Reswap）。
func invalid(w http.ResponseWriter, r *http.Request, target string, form templ.Component) {
	w.Header().Set("HX-Retarget", target)
	w.Header().Set("HX-Reswap", "innerHTML")
	render(w, r, http.StatusUnprocessableEntity, form)
}
