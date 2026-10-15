package shell

import (
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/gui/service"
)

type fakeSender struct {
	ok    bool
	sent  int
	title string
	body  string
}

func (f *fakeSender) Send(title, body string) bool {
	f.sent++
	f.title, f.body = title, body
	return f.ok
}

type fakeBackendSource struct {
	back service.Backend
	ok   bool
}

func (f fakeBackendSource) Backend() (service.Backend, bool) { return f.back, f.ok }

func newNotice(shown, daemonReady, sendOK bool) (*TrayNotice, *fakeSender, *fakeConfigManager) {
	cfg := &fakeConfigManager{}
	cfg.cfg.GUI.TrayNoticeShown = shown
	s := &fakeSender{ok: sendOK}
	src := fakeBackendSource{back: service.Backend{Config: cfg}, ok: daemonReady}
	return NewTrayNotice(src, s), s, cfg
}

func TestTrayNotice_FirstTimeSendsAndSaves(t *testing.T) {
	n, s, cfg := newNotice(false, true, true)

	n.Show()

	if s.sent != 1 || s.body == "" {
		t.Errorf("通知 = %d 回 %q, want 1 回", s.sent, s.body)
	}
	if !cfg.GetConfig().GUI.TrayNoticeShown {
		t.Error("gui.tray_notice_shown を保存していない")
	}

	n.Show()
	if s.sent != 1 {
		t.Errorf("2 回目の非表示で再通知した: %d", s.sent)
	}
}

func TestTrayNotice_Skips(t *testing.T) {
	tests := []struct {
		name                   string
		shown, ready, sendOK   bool
		wantSent, wantSaveFlag bool
	}{
		{"表示済み", true, true, true, false, true},
		{"デーモン未起動", false, false, true, false, false},
		{"OS 通知を使えない（次回に持ち越す）", false, true, false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, s, cfg := newNotice(tt.shown, tt.ready, tt.sendOK)

			n.Show()

			if (s.sent == 1) != tt.wantSent {
				t.Errorf("sent = %d, want 送信 = %v", s.sent, tt.wantSent)
			}
			if cfg.GetConfig().GUI.TrayNoticeShown != tt.wantSaveFlag {
				t.Errorf("TrayNoticeShown = %v, want %v", cfg.GetConfig().GUI.TrayNoticeShown, tt.wantSaveFlag)
			}
		})
	}
}

var _ core.ConfigManager = (*fakeConfigManager)(nil)
