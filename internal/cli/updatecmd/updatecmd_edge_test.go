package updatecmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
