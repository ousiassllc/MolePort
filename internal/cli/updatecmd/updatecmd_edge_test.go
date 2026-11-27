package updatecmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core/update"
	"github.com/ousiassllc/moleport/internal/daemon/control"
)

func TestRunUpdate_CheckOnly_NoReleaseURL(t *testing.T) {
	stubExit(t)
	stubVersion(t, "v0.1.0")

	// ReleaseURL が空の場合、URL が出力されないことを確認
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(githubRelease{
			TagName: "v0.3.0",
			HTMLURL: "", // 空の URL
		})
	}))
	defer srv.Close()
	stubVersionChecker(t, srv.URL)

	output := captureStdout(t, func() {
		RunUpdate(t.TempDir(), []string{"--check"})
	})

	if !strings.Contains(output, "v0.3.0") {
		t.Errorf("output = %q, should contain latest version v0.3.0", output)
	}
	// 空 URL の場合は "https://" が出力されないことを確認
	if strings.Contains(output, "https://") {
		t.Errorf("output = %q, should not contain a URL when ReleaseURL is empty", output)
	}
}

func TestRunUpdate_DevBuild_WithArgs(t *testing.T) {
	stubExit(t)
	stubVersion(t, "dev")

	// --check フラグがあっても dev ビルドではエラー終了する
	code, stderr := captureExit(t, func() {
		RunUpdate(t.TempDir(), []string{"--check"})
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "dev build") {
		t.Errorf("stderr = %q, should mention dev build", stderr)
	}
}

func TestRunUpdate_CheckOnly_UnknownArgsIgnored(t *testing.T) {
	stubExit(t)
	stubVersion(t, "v0.1.0")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(githubRelease{
			TagName: "v0.5.0",
			HTMLURL: "https://example.com/release",
		})
	}))
	defer srv.Close()
	stubVersionChecker(t, srv.URL)

	// --check 以外の引数は無視されて --check モードで動作する
	output := captureStdout(t, func() {
		RunUpdate(t.TempDir(), []string{"--unknown", "--check"})
	})

	if !strings.Contains(output, "v0.5.0") {
		t.Errorf("output = %q, should contain latest version", output)
	}
}

func TestRunUpdate_NilResult(t *testing.T) {
	stubExit(t)
	stubVersion(t, "v0.1.0")

	// 空レスポンス（JSON デコードが nil/ゼロ値になるケース）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// tag_name が空 → VersionChecker は result==nil を返す
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "", HTMLURL: ""})
	}))
	defer srv.Close()
	stubVersionChecker(t, srv.URL)

	output := captureStdout(t, func() {
		RunUpdate(t.TempDir(), nil)
	})

	// result==nil || !result.UpdateAvailable のパスを通り「最新です」と出力される
	if !strings.Contains(output, "up to date") {
		t.Errorf("output = %q, should contain 'up to date'", output)
	}
}

func TestRunUpdate_UpdateAvailable_NoCheckFlag(t *testing.T) {
	stubExit(t)
	stubVersion(t, "v0.1.0")

	// アップデートが利用可能だが --check なし → フルアップデートフローに進む
	// フルアップデートはアセットダウンロードで失敗するが、
	// 「available」メッセージと「downloading」メッセージが出力されることを確認
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(githubRelease{
			TagName: "v0.9.0",
			HTMLURL: "https://example.com/release",
		})
	}))
	defer srv.Close()
	stubVersionChecker(t, srv.URL)

	code, stderr := captureExit(t, func() {
		// stdout もキャプチャしたいがエラー終了するため captureExit を使う
		RunUpdate(t.TempDir(), nil)
	})

	// フルアップデートはアセットが見つからずエラー終了する
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (update should fail without real assets)", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message about update failure")
	}
}

func TestRunUpdate_InvalidJSON(t *testing.T) {
	stubExit(t)
	stubVersion(t, "v0.1.0")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer srv.Close()
	stubVersionChecker(t, srv.URL)

	code, stderr := captureExit(t, func() {
		RunUpdate(t.TempDir(), nil)
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message for invalid JSON")
	}
}

func TestRunUpdate_NetworkError(t *testing.T) {
	stubExit(t)
	stubVersion(t, "v0.1.0")

	// 即座に閉じたサーバーへの接続でネットワークエラーを再現
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srvURL := srv.URL
	srv.Close() // 即座に閉じる

	stubVersionChecker(t, srvURL)

	code, stderr := captureExit(t, func() {
		RunUpdate(t.TempDir(), nil)
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr should contain an error message for network error")
	}
}

// updateFlowStubs は update 本体とデーモン操作を差し替えた RunUpdate の呼び出し記録。
type updateFlowStubs struct {
	takeoverDirs []string
	starts       int
}

// stubUpdateFlow は更新可能なリリースを返すサーバーと、デーモン稼働中の状態を用意する。
// apply は applyUpdate の差し替え（beforeReplace を呼ぶかどうかを制御できる）、takeoverErr は Takeover の結果。
func stubUpdateFlow(t *testing.T, takeoverErr error,
	apply func(beforeReplace func() error) error,
) *updateFlowStubs {
	t.Helper()
	stubExit(t)
	stubVersion(t, "v0.1.0")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v0.9.0", HTMLURL: "https://example.com/release"})
	}))
	t.Cleanup(srv.Close)
	stubVersionChecker(t, srv.URL)

	stubs := &updateFlowStubs{}
	origRunning, origTakeover, origStart, origApply := isDaemonRunning, takeoverDaemon, startDaemonProcess, applyUpdate
	t.Cleanup(func() {
		isDaemonRunning, takeoverDaemon, startDaemonProcess, applyUpdate = origRunning, origTakeover, origStart, origApply
	})
	isDaemonRunning = func(string) bool { return true }
	takeoverDaemon = func(configDir string, _ time.Duration) (int, error) {
		stubs.takeoverDirs = append(stubs.takeoverDirs, configDir)
		return 1234, takeoverErr
	}
	startDaemonProcess = func(string) (int, error) {
		stubs.starts++
		return 5678, nil
	}
	applyUpdate = func(_ context.Context, _ *update.VersionChecker, beforeReplace func() error, _ func(string)) error {
		return apply(beforeReplace)
	}
	return stubs
}

func TestRunUpdate_DaemonStoppedBeforeReplaceAndRestarted(t *testing.T) {
	stubs := stubUpdateFlow(t, nil, func(beforeReplace func() error) error { return beforeReplace() })
	dir := t.TempDir()

	output := captureStdout(t, func() { RunUpdate(dir, nil) })

	if len(stubs.takeoverDirs) != 1 || stubs.takeoverDirs[0] != dir {
		t.Errorf("Takeover calls = %v, want one call with %q", stubs.takeoverDirs, dir)
	}
	if stubs.starts != 1 {
		t.Errorf("StartDaemonProcess calls = %d, want 1", stubs.starts)
	}
	if !strings.Contains(output, "v0.9.0") {
		t.Errorf("output = %q, should report the updated version", output)
	}
}

// 停止の完了を確認できない場合は更新を中断し、再起動（2 つ目のデーモン起動）をしない。
func TestRunUpdate_AbortsWhenDaemonDoesNotStop(t *testing.T) {
	stubs := stubUpdateFlow(t, control.ErrTakeoverTimeout, func(beforeReplace func() error) error { return beforeReplace() })

	code, stderr := captureExit(t, func() { RunUpdate(t.TempDir(), nil) })

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "daemon kill") {
		t.Errorf("stderr = %q, should suggest 'daemon kill'", stderr)
	}
	if stubs.starts != 0 {
		t.Errorf("StartDaemonProcess calls = %d, want 0 (daemon was not stopped)", stubs.starts)
	}
}

// デーモン停止前（検証など）に更新が失敗した場合は、デーモンを再起動しない。
func TestRunUpdate_NoRestartWhenUpdateFailsBeforeStop(t *testing.T) {
	stubs := stubUpdateFlow(t, nil, func(func() error) error { return errors.New("verify checksum: mismatch") })

	code, _ := captureExit(t, func() { RunUpdate(t.TempDir(), nil) })

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if len(stubs.takeoverDirs) != 0 {
		t.Errorf("Takeover calls = %v, want none", stubs.takeoverDirs)
	}
	if stubs.starts != 0 {
		t.Errorf("StartDaemonProcess calls = %d, want 0", stubs.starts)
	}
}

// デーモン停止後にバイナリ置換が失敗した場合は、停止したデーモンを再起動する。
func TestRunUpdate_RestartsWhenReplaceFailsAfterStop(t *testing.T) {
	stubs := stubUpdateFlow(t, nil, func(beforeReplace func() error) error {
		if err := beforeReplace(); err != nil {
			return err
		}
		return errors.New("replace binary: permission denied")
	})

	code, _ := captureExit(t, func() { RunUpdate(t.TempDir(), nil) })

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stubs.starts != 1 {
		t.Errorf("StartDaemonProcess calls = %d, want 1", stubs.starts)
	}
}
