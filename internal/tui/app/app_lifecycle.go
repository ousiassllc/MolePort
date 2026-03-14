package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

// refreshForwardPanel はフォワードパネルを最新のセッション情報で更新する。
func (m *MainModel) refreshForwardPanel() {
	m.dashboard.SetForwardSessions(m.sessions)
}

// shutdown はアプリケーションを終了する。
func (m *MainModel) shutdown() tea.Cmd {
	m.quitting = true
	// IPC クライアントをクリーンアップ（daemon は停止しない）
	if m.subscriptionID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), ipcShutdownTimeout)
		defer cancel()
		_ = m.client.Unsubscribe(ctx, m.subscriptionID) // ベストエフォート: シャットダウン中のため失敗しても無視
	}
	return tea.Quit
}
