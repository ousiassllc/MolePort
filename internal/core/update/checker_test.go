package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestServer は GitHub API をモックする httptest サーバーを返す。
// リクエストパスと User-Agent ヘッダーを検証する。
func newTestServer(tag, htmlURL string, statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/ousiassllc/moleport/releases/latest" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "MolePort/") {
			http.Error(w, "unexpected User-Agent: "+ua, http.StatusBadRequest)
			return
		}
		if statusCode != http.StatusOK {
			w.WriteHeader(statusCode)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(githubRelease{
			TagName: tag,
			HTMLURL: htmlURL,
		})
	}))
}

// newTestChecker はテスト用の VersionChecker を生成する。
func newTestChecker(serverURL, currentVersion string, enabled bool) *VersionChecker {
	vc := New(currentVersion, enabled, time.Hour)
	vc.apiBase = serverURL
	return vc
}

func TestVersionChecker_UpdateAvailable(t *testing.T) {
	srv := newTestServer("v0.2.0", "https://github.com/ousiassllc/moleport/releases/tag/v0.2.0", http.StatusOK)
	defer srv.Close()

	vc := newTestChecker(srv.URL, "v0.1.0", true)

	result, err := vc.LatestVersion(t.Context())
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if result == nil {
		t.Fatal("LatestVersion() returned nil")
	}
	if !result.UpdateAvailable {
		t.Error("UpdateAvailable should be true when latest > current")
	}
	if result.LatestVersion != "v0.2.0" {
		t.Errorf("LatestVersion = %q, want %q", result.LatestVersion, "v0.2.0")
	}
	if !vc.UpdateAvailable() {
		t.Error("UpdateAvailable() should return true")
	}
}

func TestVersionChecker_NoUpdate(t *testing.T) {
	srv := newTestServer("v0.2.0", "https://github.com/ousiassllc/moleport/releases/tag/v0.2.0", http.StatusOK)
	defer srv.Close()

	vc := newTestChecker(srv.URL, "v0.2.0", true)

	result, err := vc.LatestVersion(t.Context())
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if result.UpdateAvailable {
		t.Error("UpdateAvailable should be false when latest == current")
	}
	if vc.UpdateAvailable() {
		t.Error("UpdateAvailable() should return false")
	}
}

func TestVersionChecker_CurrentNewer(t *testing.T) {
	srv := newTestServer("v0.2.0", "https://github.com/ousiassllc/moleport/releases/tag/v0.2.0", http.StatusOK)
	defer srv.Close()

	vc := newTestChecker(srv.URL, "v0.3.0", true)

	result, err := vc.LatestVersion(t.Context())
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if result.UpdateAvailable {
		t.Error("UpdateAvailable should be false when current > latest")
	}
}

func TestVersionChecker_NetworkError(t *testing.T) {
	srv := newTestServer("", "", http.StatusInternalServerError)
	defer srv.Close()

	vc := newTestChecker(srv.URL, "v0.1.0", true)

	_, err := vc.LatestVersion(t.Context())
	if err == nil {
		t.Fatal("LatestVersion() should return error on 500 response")
	}
}

func TestVersionChecker_DevSkip(t *testing.T) {
	vc := New("dev", true, time.Hour)

	result, err := vc.LatestVersion(t.Context())
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if result != nil {
		t.Error("LatestVersion() should return nil for dev version")
	}
	if vc.UpdateAvailable() {
		t.Error("UpdateAvailable() should return false for dev version")
	}
}

func TestVersionChecker_DisabledSkip(t *testing.T) {
	vc := New("v0.1.0", false, time.Hour)

	result, err := vc.LatestVersion(t.Context())
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if result != nil {
		t.Error("LatestVersion() should return nil when disabled")
	}
	if vc.UpdateAvailable() {
		t.Error("UpdateAvailable() should return false when disabled")
	}
}

func TestVersionChecker_CacheBehavior(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(githubRelease{
			TagName: "v0.2.0",
			HTMLURL: "https://github.com/ousiassllc/moleport/releases/tag/v0.2.0",
		})
	}))
	defer srv.Close()

	vc := newTestChecker(srv.URL, "v0.1.0", true)

	// 1回目: API を呼ぶ
	_, err := vc.LatestVersion(t.Context())
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if callCount.Load() != 1 {
		t.Fatalf("expected 1 API call, got %d", callCount.Load())
	}

	// 2回目: キャッシュを使う
	_, err = vc.LatestVersion(t.Context())
	if err != nil {
		t.Fatalf("LatestVersion() error = %v", err)
	}
	if callCount.Load() != 1 {
		t.Errorf("expected 1 API call (cached), got %d", callCount.Load())
	}
}

func TestVersionChecker_StartStop(t *testing.T) {
	srv := newTestServer("v0.2.0", "https://github.com/ousiassllc/moleport/releases/tag/v0.2.0", http.StatusOK)
	defer srv.Close()

	vc := newTestChecker(srv.URL, "v0.1.0", true)

	// Start は二重起動しない
	vc.Start(t.Context(), 10*time.Millisecond)
	vc.Start(t.Context(), 10*time.Millisecond) // 二重呼び出し: 無視される

	// 初回チェックが完了するまで待つ
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if vc.UpdateAvailable() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !vc.UpdateAvailable() {
		t.Error("expected update to be available after Start")
	}

	// Stop は冪等
	vc.Stop()
	vc.Stop()

	// dev バージョンでは Start しない
	devVC := New("dev", true, time.Hour)
	devVC.Start(t.Context(), 0)
	devVC.Stop()
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{"v0.1.0", "v0.2.0", true},
		{"v0.2.0", "v0.2.0", false},
		{"v0.3.0", "v0.2.0", false},
		{"0.1.0", "0.2.0", true},         // "v" プレフィックスなし
		{"v1.0.0", "v1.0.1", true},       // パッチバージョン
		{"v1.0.0", "v2.0.0", true},       // メジャーバージョン
		{"invalid", "v0.2.0", false},     // 不正なバージョン
		{"v0.1.0", "invalid", false},     // 不正なバージョン
		{"v0.1.0-alpha", "v0.1.0", true}, // プレリリース
	}

	for _, tt := range tests {
		t.Run(tt.current+"_vs_"+tt.latest, func(t *testing.T) {
			got := isNewerVersion(tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("isNewerVersion(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}
