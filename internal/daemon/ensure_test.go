package daemon

import (
	"errors"
	"strings"
	"testing"
)

func TestEnsureDaemon_AutoStartFailure(t *testing.T) {
	dir := t.TempDir()

	orig := startDaemonFunc
	startDaemonFunc = func(configDir string) (int, error) {
		return 0, errors.New("mock start failure")
	}
	defer func() { startDaemonFunc = orig }()

	_, err := EnsureDaemon(dir)
	if err == nil {
		t.Fatal("EnsureDaemon() should return error when auto-start fails")
	}
	if got := err.Error(); got != "failed to auto-start daemon: mock start failure" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestEnsureDaemon_AlreadyRunning_ConnectFails(t *testing.T) {
	dir := t.TempDir()

	// PIDファイルを作成し、自プロセスのPIDを書き込む（デーモン稼働中と見せかける）
	pf := NewPIDFile(PIDFilePath(dir))
	if err := pf.Acquire(); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer func() { _ = pf.Release() }()

	// ソケットが存在しないため接続に失敗するはず
	_, err := EnsureDaemon(dir)
	if err == nil {
		t.Fatal("EnsureDaemon() should return error when connect fails")
	}
	if !strings.Contains(err.Error(), "failed to connect to daemon") {
		t.Errorf("unexpected error: %s", err)
	}
}
