package daemon

import (
	"path/filepath"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/infra"
	"github.com/ousiassllc/moleport/internal/infra/yamlstore"
)

// LogConfig はデーモンのログ設定を保持する。
type LogConfig struct {
	Path  string
	Level string
}

// ResolveLogConfig は設定ファイルからログファイルのパスとレベルを解決する。
// 設定の読み込みに失敗した場合はデフォルトの設定を使用する。
func ResolveLogConfig(configDir string) LogConfig {
	store := yamlstore.NewYAMLStore()
	cfgMgr := core.NewConfigManager(store, configDir)
	cfg, err := cfgMgr.LoadConfig()
	if err != nil {
		c := core.DefaultConfig()
		cfg = &c
	}
	logPath := cfg.Log.File
	if expanded, err := infra.ExpandTilde(logPath); err == nil {
		logPath = expanded
	}
	return LogConfig{Path: logPath, Level: cfg.Log.Level}
}

// SocketPath はデーモンの Unix ソケットパスを返す。
func SocketPath(configDir string) string {
	return filepath.Join(configDir, "moleport.sock")
}

// PIDFilePath はデーモンの PID ファイルパスを返す。
func PIDFilePath(configDir string) string {
	return filepath.Join(configDir, "moleport.pid")
}
