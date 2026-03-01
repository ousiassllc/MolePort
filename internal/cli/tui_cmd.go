package cli

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/i18n"
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
			exitError("%s", i18n.T("cli.tui.daemon_start_failed", map[string]any{"Error": err}))
		}
		fmt.Println(i18n.T("cli.tui.daemon_started", map[string]any{"PID": pid}))
	}

	// リトライ付きで接続
	client, err := daemon.EnsureDaemonWithRetry(configDir, 5*time.Second)
	if err != nil {
		exitError("%s", i18n.T("cli.tui.daemon_connect_failed", map[string]any{"Error": err}))
	}

	// Bubble Tea プログラム起動
	model := app.NewMainModel(client, Version, configDir)
	p := tea.NewProgram(model, tea.WithAltScreen())

	// TUI クレデンシャルハンドラーを設定
	client.SetCredentialHandler(app.NewTUICredentialHandler(p))

	if _, err := p.Run(); err != nil {
		client.Close()
		exitError("%s", i18n.T("cli.tui.tui_error", map[string]any{"Error": err}))
	}

	client.Close()
}
