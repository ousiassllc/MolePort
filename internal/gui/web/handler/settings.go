package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/service/settings"
	"github.com/ousiassllc/moleport/internal/gui/web/view/organisms"
	"github.com/ousiassllc/moleport/internal/i18n"
)

const (
	eventHeader          = "mp:" + string(service.TopicHeader)
	eventAll             = "mp:" + string(service.TopicAll)
	eventSettingsApplied = "mp:settings-applied"
)

// Settings は設定画面（/fragments/settings, POST /actions/settings）と POST /actions/update/open。
type Settings struct {
	set settings.Settings
}

// NewSettings は Settings を返す。
func NewSettings(set settings.Settings) *Settings { return &Settings{set: set} }

// Form は GET /fragments/settings。保存済みの設定でフォームを返す。
func (s *Settings) Form(w http.ResponseWriter, r *http.Request) {
	v, err := s.set.View()
	if err != nil {
		problem(w, r, err, eventHeader, "")
		return
	}
	render(w, r, http.StatusOK, organisms.SettingsForm(v, nil))
}

// Save は POST /actions/settings。検証エラーは 422 でフォームを再描画する。
// 成功時は全フラグメントを再取得させ（mp:all）、<html> の lang / data-theme / data-accent を
// mp:settings-applied の値で書き換えさせる（HX-Refresh・全画面遷移を使わない）。
func (s *Settings) Save(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	in := service.SettingsView{
		Language:           r.PostForm.Get("language"),
		ThemeBase:          r.PostForm.Get("theme_base"),
		ThemeAccent:        r.PostForm.Get("theme_accent"),
		UpdateCheckEnabled: r.PostForm.Get("update_check_enabled") == "on",
	}
	errs, err := s.set.Save(in)
	if err != nil {
		problem(w, r, err, eventHeader, "")
		return
	}
	if len(errs) > 0 {
		invalid(w, r, "#modal-root", organisms.SettingsForm(in, errs))
		return
	}
	trigger, err := json.Marshal(map[string]any{
		eventCloseModal: "",
		eventAll:        "",
		eventSettingsApplied: map[string]string{
			"lang": in.Language, "theme": in.ThemeBase, "accent": in.ThemeAccent,
		},
	})
	if err != nil {
		slog.Error("gui: encode HX-Trigger", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", string(trigger))
	render(w, r, http.StatusOK, toastOOB(service.ToastSuccess, i18n.T("gui.settings.saved")))
}

// OpenUpdate は POST /actions/update/open。リリースページを開く（URL はサーバー側のキャッシュから取得する）。
// 成功は 204、開くべき更新がなければ 409（ヘッダーを再取得させてバッジを消す）。
func (s *Settings) OpenUpdate(w http.ResponseWriter, r *http.Request) {
	err := s.set.OpenUpdate()
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, settings.ErrNoUpdate):
		problem(w, r, service.ErrStateConflict, eventHeader, i18n.T("gui.update.none"))
	default:
		problem(w, r, err, eventHeader, "")
	}
}
