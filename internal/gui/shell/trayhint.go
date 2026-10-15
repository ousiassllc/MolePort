package shell

import (
	"sync"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// TrayNotice は初回にウィンドウをトレイへ格納した時だけ「MolePort はトレイで稼働しています」を OS 通知する（UC-32）。
// 表示済みは config.yaml の gui.tray_notice_shown で管理する。
type TrayNotice struct {
	src    service.BackendSource
	sender TrayNoticeSender

	mu sync.Mutex // 通知と保存の二重実行を防ぐ
}

// NewTrayNotice は TrayNotice を返す。
func NewTrayNotice(src service.BackendSource, sender TrayNoticeSender) *TrayNotice {
	return &TrayNotice{src: src, sender: sender}
}

// Show は未表示なら通知を送り、gui.tray_notice_shown を保存する。
// 内蔵デーモン未起動（設定を参照・保存できない）や OS 通知を利用できない環境では何もせず、次回に持ち越す。
func (n *TrayNotice) Show() {
	b, ok := n.src.Backend()
	if !ok || b.Config == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if b.Config.GetConfig().GUI.TrayNoticeShown {
		return
	}
	if !n.sender.Send(i18n.T("gui.lifecycle.tray_notice_title"), i18n.T("gui.lifecycle.tray_notice")) {
		return
	}
	// 保存に失敗しても通知は出済み。次回また出るだけなので続行する。
	_ = b.Config.UpdateConfig(func(c *core.Config) { c.GUI.TrayNoticeShown = true })
}
