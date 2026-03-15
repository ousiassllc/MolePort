package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ousiassllc/moleport/internal/daemon"
	"github.com/ousiassllc/moleport/internal/i18n"
	"github.com/ousiassllc/moleport/internal/ipc/client"
)

// ResolveConfigDir は設定ディレクトリを解決する。
// 優先順位: flagValue > 環境変数 MOLEPORT_CONFIG_DIR > ~/.config/moleport/
func ResolveConfigDir(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	if envDir := os.Getenv("MOLEPORT_CONFIG_DIR"); envDir != "" {
		return envDir
	}

	// XDG_CONFIG_HOME を優先
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "moleport")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".config", "moleport")
}

// ConnectDaemon はデーモンに接続し、IPCClient を返す。
// 接続に失敗した場合はエラーメッセージを表示して終了する。
// テスト時に差し替え可能にするため変数として定義する（ExitFunc と同パターン）。
// NOTE: stubConnectDaemon を使用するテストは t.Parallel() と併用不可。
var ConnectDaemon = defaultConnectDaemon

func defaultConnectDaemon(configDir string) *client.IPCClient {
	c, err := daemon.EnsureDaemon(configDir)
	if err != nil {
		ExitError("%s", i18n.T("cli.error.daemon_not_running"))
	}
	return c
}

// defaultCallTimeout は RPC 呼び出しのデフォルトタイムアウト。
const defaultCallTimeout = 10 * time.Second

// CallCtx は RPC 呼び出し用のコンテキストを生成する。
func CallCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultCallTimeout)
}

// DaemonCall はデーモンに接続し、RPC 呼び出し用のコンテキストとクリーンアップ関数を返す。
func DaemonCall(configDir string) (cl *client.IPCClient, ctx context.Context, cleanup func()) {
	cl = ConnectDaemon(configDir)
	ctx, cancel := CallCtx()
	cleanup = func() {
		cancel()
		_ = cl.Close()
	}
	return
}

// ExitFunc はプロセス終了関数。テスト時に差し替えて os.Exit を回避可能にする。
// NOTE: stubExit/captureExit を使用するテストは t.Parallel() と併用不可。
var ExitFunc = os.Exit

// ExitError はエラーメッセージを stderr に出力し、終了コード 1 で終了する。
func ExitError(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s: %s\n", i18n.T("cli.error.prefix"), msg)
	ExitFunc(1)
}

// PrintJSON は値を整形された JSON として stdout に出力する。
func PrintJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		ExitError("%s", i18n.T("cli.error.json_output_failed", map[string]any{"Error": err}))
	}
}

// ParseGlobalFlags は os.Args からグローバルフラグを解析する。
// --config-dir フラグの値と残りの引数を返す。
func ParseGlobalFlags() (configDir string, args []string) {
	rawArgs := os.Args[1:]
	for i := 0; i < len(rawArgs); i++ {
		if rawArgs[i] == "--config-dir" && i+1 < len(rawArgs) {
			configDir = rawArgs[i+1]
			i++ // skip next arg (value)
			continue
		}
		if v, ok := strings.CutPrefix(rawArgs[i], "--config-dir="); ok {
			configDir = v
			continue
		}
		args = append(args, rawArgs[i])
	}
	return configDir, args
}
