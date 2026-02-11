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
	"github.com/ousiassllc/moleport/internal/ipc"
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

// connectDaemon はデーモンに接続し、IPCClient を返す。
// 接続に失敗した場合はエラーメッセージを表示して終了する。
func connectDaemon(configDir string) *ipc.IPCClient {
	client, err := daemon.EnsureDaemon(configDir)
	if err != nil {
		exitError("デーモンが稼働していません。moleport daemon start で起動してください。")
	}
	return client
}

// callCtx は RPC 呼び出し用のコンテキストを生成する（10秒タイムアウト）。
func callCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

// exitError はエラーメッセージを stderr に出力し、終了コード 1 で終了する。
func exitError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "エラー: "+format+"\n", args...)
	os.Exit(1)
}

// printJSON は値を整形された JSON として stdout に出力する。
func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		exitError("JSON 出力に失敗しました: %v", err)
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
		if strings.HasPrefix(rawArgs[i], "--config-dir=") {
			configDir = strings.TrimPrefix(rawArgs[i], "--config-dir=")
			continue
		}
		args = append(args, rawArgs[i])
	}
	return configDir, args
}
