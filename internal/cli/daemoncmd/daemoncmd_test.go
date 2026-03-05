package daemoncmd

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ousiassllc/moleport/internal/cli"
	"gopkg.in/yaml.v3"
)

type exitCalled struct{ code int }

func stubExit(t *testing.T) {
	t.Helper()
	orig := cli.ExitFunc
	t.Cleanup(func() { cli.ExitFunc = orig })
	cli.ExitFunc = func(c int) { panic(exitCalled{code: c}) }
}

func captureExit(t *testing.T, fn func()) (code int, stderr string) {
	t.Helper()
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = w.Close()
		_ = r.Close()
		os.Stderr = origStderr
	})
	os.Stderr = w
	code = -1
	func() {
		defer func() {
			if v := recover(); v != nil {
				if ec, ok := v.(exitCalled); ok {
					code = ec.code
				} else {
					panic(v)
				}
			}
		}()
		fn()
	}()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return code, buf.String()
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func writeFakePID(t *testing.T, configDir string) {
	t.Helper()
	pidPath := filepath.Join(configDir, "moleport.pid")
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0600); err != nil {
		t.Fatalf("write PID file: %v", err)
	}
}

func TestSetupDaemonLogging_DefaultLogPath(t *testing.T) {
	tmpDir := t.TempDir()
	f, err := setupDaemonLogging(tmpDir)
	if err != nil {
		t.Fatalf("setupDaemonLogging() error = %v", err)
	}
	defer func() { _ = f.Close() }()

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
	f, err := setupDaemonLogging(tmpDir)
	if err != nil {
		t.Fatalf("setupDaemonLogging() error = %v", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("expected log file at %s, but it does not exist", logPath)
	}
}

func TestRunDaemon_SubcommandRequired(t *testing.T) {
	stubExit(t)
	code, _ := captureExit(t, func() { RunDaemon("/tmp", []string{}) })
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunDaemon_UnknownSubcommand(t *testing.T) {
	stubExit(t)
	code, _ := captureExit(t, func() { RunDaemon("/tmp", []string{"unknown"}) })
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunDaemonStart_NoPanic(t *testing.T) {
	stubExit(t)
	captureExit(t, func() { runDaemonStart(t.TempDir()) })
}

func TestRunDaemonStop_NotRunning(t *testing.T) {
	output := captureStdout(t, func() { runDaemonStop(t.TempDir(), []string{}) })
	if output == "" {
		t.Error("runDaemonStop should produce output when daemon is not running")
	}
}

func TestRunDaemonStop_InvalidFlag(t *testing.T) {
	stubExit(t)
	code, _ := captureExit(t, func() { runDaemonStop(t.TempDir(), []string{"--bad-flag"}) })
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunDaemonStatus_NotRunning(t *testing.T) {
	output := captureStdout(t, func() { runDaemonStatus(t.TempDir()) })
	if output == "" {
		t.Error("runDaemonStatus should produce output when daemon is not running")
	}
}

func TestRunDaemonKill_NotRunning(t *testing.T) {
	output := captureStdout(t, func() { runDaemonKill(t.TempDir()) })
	if output == "" {
		t.Error("runDaemonKill should produce output when daemon is not running")
	}
}

func TestRunDaemon_RoutesToStart(t *testing.T) {
	stubExit(t)
	captureExit(t, func() { RunDaemon(t.TempDir(), []string{"start"}) })
}

func TestRunDaemon_RoutesToStop(t *testing.T) {
	output := captureStdout(t, func() { RunDaemon(t.TempDir(), []string{"stop"}) })
	if output == "" {
		t.Error("RunDaemon stop should produce output")
	}
}

func TestRunDaemon_RoutesToStatus(t *testing.T) {
	output := captureStdout(t, func() { RunDaemon(t.TempDir(), []string{"status"}) })
	if output == "" {
		t.Error("RunDaemon status should produce output")
	}
}

func TestRunDaemon_RoutesToKill(t *testing.T) {
	output := captureStdout(t, func() { RunDaemon(t.TempDir(), []string{"kill"}) })
	if output == "" {
		t.Error("RunDaemon kill should produce output")
	}
}

func TestRunDaemonStop_RunningButNoSocket(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()
	writeFakePID(t, configDir)
	code, stderr := captureExit(t, func() { runDaemonStop(configDir, []string{}) })
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}

func TestRunDaemonStatus_RunningButNoSocket(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()
	writeFakePID(t, configDir)
	code, stderr := captureExit(t, func() { runDaemonStatus(configDir) })
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message")
	}
}

func TestRunDaemonKill_RunningProcess(t *testing.T) {
	stubExit(t)
	configDir := t.TempDir()
	pidPath := filepath.Join(configDir, "moleport.pid")
	if err := os.WriteFile(pidPath, []byte("999999999"), 0600); err != nil {
		t.Fatalf("write PID file: %v", err)
	}
	output := captureStdout(t, func() { runDaemonKill(configDir) })
	if output == "" {
		t.Error("runDaemonKill should produce output")
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
