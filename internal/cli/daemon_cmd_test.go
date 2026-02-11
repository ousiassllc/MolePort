package cli

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSetupDaemonLogging_DefaultLogPath(t *testing.T) {
	// 一時ディレクトリを config ディレクトリとして使用
	tmpDir := t.TempDir()

	if err := setupDaemonLogging(tmpDir); err != nil {
		t.Fatalf("setupDaemonLogging() error = %v", err)
	}

	// デフォルト設定では log.file = "~/.config/moleport/moleport.log"
	// ただし configDir 内にコンフィグがないのでデフォルトが使われる
	// デフォルトパスは ~ を含むため、展開後のパスにファイルが作成される
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home dir")
	}
	expectedPath := filepath.Join(home, ".config", "moleport", "moleport.log")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected log file at %s, but it does not exist", expectedPath)
	}
}

func TestSetupDaemonLogging_CustomLogPath(t *testing.T) {
	tmpDir := t.TempDir()

	// カスタム設定ファイルを作成
	logPath := filepath.Join(tmpDir, "custom.log")
	cfg := map[string]any{
		"log": map[string]any{
			"level": "debug",
			"file":  logPath,
		},
	}
	cfgData, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), cfgData, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := setupDaemonLogging(tmpDir); err != nil {
		t.Fatalf("setupDaemonLogging() error = %v", err)
	}

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("expected log file at %s, but it does not exist", logPath)
	}
}

func TestParseSlogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseSlogLevel(tt.input)
			if got != tt.want {
				t.Errorf("parseSlogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
