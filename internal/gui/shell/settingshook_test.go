package shell

import (
	"errors"
	"testing"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/service/settings"
)

type fakeSettings struct {
	settings.Settings
	errs service.FormErrors
	err  error
}

func (f fakeSettings) Save(service.SettingsView) (service.FormErrors, error) { return f.errs, f.err }

func TestSettingsHook_Save(t *testing.T) {
	tests := []struct {
		name       string
		inner      fakeSettings
		wantCalled int
	}{
		{"保存成功", fakeSettings{}, 1},
		{"検証エラー", fakeSettings{errs: service.FormErrors{"language": "bad"}}, 0},
		{"保存失敗", fakeSettings{err: errors.New("boom")}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := 0
			h := settingsHook{Settings: tt.inner, onSaved: func() { called++ }}

			errs, err := h.Save(service.SettingsView{})

			if called != tt.wantCalled {
				t.Errorf("onSaved = %d 回, want %d", called, tt.wantCalled)
			}
			if len(errs) != len(tt.inner.errs) || err != tt.inner.err {
				t.Errorf("結果を素通ししていない: %v, %v", errs, err)
			}
		})
	}
}
