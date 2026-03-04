package cli

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
)

// captureStdout は fn 実行中の stdout 出力をキャプチャして返す。
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func TestRunVersion_BasicOutput(t *testing.T) {
	// デーモンが稼働していない一時ディレクトリを使用
	configDir := t.TempDir()

	output := captureStdout(t, func() {
		RunVersion(configDir, []string{})
	})

	// バージョン行が出力されること
	if !strings.Contains(output, "MolePort") {
		t.Errorf("output should contain 'MolePort', got %q", output)
	}
	if !strings.Contains(output, Version) {
		t.Errorf("output should contain version %q, got %q", Version, output)
	}
	if !strings.Contains(output, runtime.Version()) {
		t.Errorf("output should contain Go version %q, got %q", runtime.Version(), output)
	}
	if !strings.Contains(output, runtime.GOOS) {
		t.Errorf("output should contain GOOS %q, got %q", runtime.GOOS, output)
	}
}

func TestRunVersion_NoDaemon_NoPanic(t *testing.T) {
	// PID ファイルが存在しない configDir を使用
	configDir := t.TempDir()

	// パニックせずに正常終了すること
	output := captureStdout(t, func() {
		RunVersion(configDir, []string{})
	})

	// バージョン行のみ出力される（エラーメッセージなし）
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line of output, got %d: %q", len(lines), output)
	}
}
