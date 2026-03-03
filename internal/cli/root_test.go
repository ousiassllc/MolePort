package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveConfigDir_FlagValue(t *testing.T) {
	got := ResolveConfigDir("/custom/path")
	if got != "/custom/path" {
		t.Errorf("ResolveConfigDir with flag = %q, want %q", got, "/custom/path")
	}
}

func TestResolveConfigDir_EnvVar(t *testing.T) {
	t.Setenv("MOLEPORT_CONFIG_DIR", "/env/path")
	got := ResolveConfigDir("")
	if got != "/env/path" {
		t.Errorf("ResolveConfigDir with env = %q, want %q", got, "/env/path")
	}
}

func TestResolveConfigDir_XDGConfigHome(t *testing.T) {
	t.Setenv("MOLEPORT_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	got := ResolveConfigDir("")
	want := filepath.Join("/xdg/config", "moleport")
	if got != want {
		t.Errorf("ResolveConfigDir with XDG = %q, want %q", got, want)
	}
}

func TestResolveConfigDir_Default(t *testing.T) {
	t.Setenv("MOLEPORT_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	got := ResolveConfigDir("")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "moleport")
	if got != want {
		t.Errorf("ResolveConfigDir default = %q, want %q", got, want)
	}
}

func TestResolveConfigDir_FlagOverridesEnv(t *testing.T) {
	t.Setenv("MOLEPORT_CONFIG_DIR", "/env/path")
	got := ResolveConfigDir("/flag/path")
	if got != "/flag/path" {
		t.Errorf("ResolveConfigDir flag should override env: got %q, want %q", got, "/flag/path")
	}
}

func TestParseGlobalFlags_NoFlags(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"moleport", "daemon", "start"}

	configDir, args := ParseGlobalFlags()
	if configDir != "" {
		t.Errorf("configDir = %q, want empty", configDir)
	}
	if len(args) != 2 || args[0] != "daemon" || args[1] != "start" {
		t.Errorf("args = %v, want [daemon start]", args)
	}
}

func TestParseGlobalFlags_WithConfigDir(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"moleport", "--config-dir", "/tmp/test", "daemon", "start"}

	configDir, args := ParseGlobalFlags()
	if configDir != "/tmp/test" {
		t.Errorf("configDir = %q, want %q", configDir, "/tmp/test")
	}
	if len(args) != 2 || args[0] != "daemon" || args[1] != "start" {
		t.Errorf("args = %v, want [daemon start]", args)
	}
}

func TestParseGlobalFlags_ConfigDirAtEnd(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"moleport", "version", "--config-dir", "/tmp/test"}

	configDir, args := ParseGlobalFlags()
	if configDir != "/tmp/test" {
		t.Errorf("configDir = %q, want %q", configDir, "/tmp/test")
	}
	if len(args) != 1 || args[0] != "version" {
		t.Errorf("args = %v, want [version]", args)
	}
}

func TestParseGlobalFlags_ConfigDirEqualsFormat(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"moleport", "--config-dir=/tmp/eq-test", "version"}

	configDir, args := ParseGlobalFlags()
	if configDir != "/tmp/eq-test" {
		t.Errorf("configDir = %q, want %q", configDir, "/tmp/eq-test")
	}
	if len(args) != 1 || args[0] != "version" {
		t.Errorf("args = %v, want [version]", args)
	}
}

func TestParseGlobalFlags_Empty(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()

	os.Args = []string{"moleport"}

	configDir, args := ParseGlobalFlags()
	if configDir != "" {
		t.Errorf("configDir = %q, want empty", configDir)
	}
	if len(args) != 0 {
		t.Errorf("args = %v, want empty", args)
	}
}

// exitCalled はテスト用 exitFunc が呼ばれたことを示す panic 型。
type exitCalled struct{ code int }

// stubExit は exitFunc を差し替えて os.Exit を回避するヘルパー。
// exitFunc が呼ばれると exitCalled を panic するので、
// captureExit で recover して終了コードを取得する。
func stubExit(t *testing.T) {
	t.Helper()
	orig := exitFunc
	t.Cleanup(func() { exitFunc = orig })
	exitFunc = func(c int) { panic(exitCalled{code: c}) }
}

// captureExit は fn を呼び、exitFunc 経由の終了コードと stderr 出力を返す。
// exitFunc が呼ばれなかった場合は code=-1 を返す。
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

func TestExitError_CallsExitFunc(t *testing.T) {
	stubExit(t)

	code, output := captureExit(t, func() {
		exitError("something went %s", "wrong")
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(output, "wrong") {
		t.Errorf("stderr = %q, want to contain %q", output, "wrong")
	}
}
