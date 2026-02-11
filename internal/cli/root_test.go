package cli

import (
	"os"
	"path/filepath"
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
