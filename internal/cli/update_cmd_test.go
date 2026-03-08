package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/core/update"
)

// githubRelease はテスト用の GitHub リリース JSON 構造体。
type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// stubVersionChecker は newVersionChecker を差し替えて、
// 指定した httptest サーバーに接続する VersionChecker を返すヘルパー。
func stubVersionChecker(t *testing.T, serverURL string) {
	t.Helper()
	orig := newVersionChecker
	t.Cleanup(func() { newVersionChecker = orig })
	newVersionChecker = func(version string) *update.VersionChecker {
		vc := update.New(version, true, 0)
		vc.SetAPIBase(serverURL)
		return vc
	}
}

// stubVersion は Version を差し替えて t.Cleanup で復元するヘルパー。
func stubVersion(t *testing.T, v string) {
	t.Helper()
	orig := Version
	t.Cleanup(func() { Version = orig })
	Version = v
}

func TestRunUpdate_DevBuild(t *testing.T) {
	stubExit(t)
	stubVersion(t, "dev")

	code, stderr := captureExit(t, func() {
		RunUpdate(t.TempDir(), nil)
	})

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "dev build") {
		t.Errorf("stderr = %q, should mention dev build", stderr)
	}
}

func TestRunUpdate_CheckFailed(t *testing.T) {
	stubExit(t)
	stubVersion(t, "v0.1.0")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
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
		t.Error("stderr should contain an error message")
	}
}

func TestRunUpdate_AlreadyLatest(t *testing.T) {
	stubExit(t)
	stubVersion(t, "v0.2.0")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(githubRelease{
			TagName: "v0.2.0",
			HTMLURL: "https://github.com/ousiassllc/moleport/releases/tag/v0.2.0",
		})
	}))
	defer srv.Close()
	stubVersionChecker(t, srv.URL)

	output := captureStdout(t, func() {
		RunUpdate(t.TempDir(), nil)
	})

	if !strings.Contains(output, "v0.2.0") {
		t.Errorf("output = %q, should contain current version", output)
	}
	if !strings.Contains(output, "up to date") {
		t.Errorf("output = %q, should contain 'up to date'", output)
	}
}

func TestRunUpdate_CheckOnly(t *testing.T) {
	stubExit(t)
	stubVersion(t, "v0.1.0")

	releaseURL := "https://github.com/ousiassllc/moleport/releases/tag/v0.3.0"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(githubRelease{
			TagName: "v0.3.0",
			HTMLURL: releaseURL,
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
	if !strings.Contains(output, releaseURL) {
		t.Errorf("output = %q, should contain release URL", output)
	}
}
