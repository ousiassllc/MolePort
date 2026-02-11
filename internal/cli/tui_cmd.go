package cli

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/tui/app"
)

// RunTUI は tui サブコマンドを実行する。
func RunTUI(configDir string, args []string) {
	// デーモンが未起動なら自動起動
	pidPath := daemon.PIDFilePath(configDir)
	running, _ := daemon.IsRunning(pidPath)
	if !running {
		pid, err := daemon.StartDaemonProcess(configDir)
		if err != nil {
			exitError("デーモンの起動に失敗しました: %v", err)
		}
		fmt.Printf("デーモンを起動しました (PID: %d)\n", pid)
	}

	// リトライ付きで接続
	client, err := daemon.EnsureDaemonWithRetry(configDir, 5*time.Second)
	if err != nil {
		exitError("デーモンへの接続に失敗しました: %v", err)
	}

	// Bubble Tea プログラム起動
	model := app.NewMainModel(client, Version)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		client.Close()
		exitError("TUI エラー: %v", err)
	}

	client.Close()
}
