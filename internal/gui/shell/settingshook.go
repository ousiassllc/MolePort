package shell

import (
	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/service/settings"
)

// settingsHook は設定の保存が成功した直後に onSaved を呼ぶ。言語変更をトレイメニューへ即時反映するために使う（UC-33）。
type settingsHook struct {
	settings.Settings
	onSaved func()
}

// Save は内側の Save が成功（検証エラーなし）した時だけ onSaved を呼ぶ。
func (h settingsHook) Save(in service.SettingsView) (service.FormErrors, error) {
	errs, err := h.Settings.Save(in)
	if err == nil && len(errs) == 0 {
		h.onSaved()
	}
	return errs, err
}
