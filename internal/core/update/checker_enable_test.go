package update

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestVersionChecker_CachedDoesNotFetch(t *testing.T) {
	var hits atomic.Int32
	srv := newTestServer("v0.2.0", "https://github.com/ousiassllc/moleport/releases/tag/v0.2.0", http.StatusOK)
	defer srv.Close()
	counting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		srv.Config.Handler.ServeHTTP(w, r)
	}))
	defer counting.Close()

	vc := newTestChecker(counting.URL, "v0.1.0", true)
	if _, ok := vc.Cached(); ok {
		t.Fatal("未チェックなのに Cached が ok")
	}
	if hits.Load() != 0 {
		t.Fatal("Cached がネットワークへアクセスした")
	}
	if _, err := vc.LatestVersion(t.Context()); err != nil {
		t.Fatal(err)
	}
	got, ok := vc.Cached()
	if !ok || !got.UpdateAvailable || got.LatestVersion != "v0.2.0" {
		t.Errorf("Cached = %+v, %v", got, ok)
	}

	vc.SetEnabled(false)
	if _, ok := vc.Cached(); ok {
		t.Error("無効化後も Cached が ok")
	}
	if hits.Load() != 1 {
		t.Errorf("アクセス回数 = %d, want 1", hits.Load())
	}
}

func TestVersionChecker_SetEnabledStartsAndStops(t *testing.T) {
	srv := newTestServer("v0.2.0", "https://github.com/ousiassllc/moleport/releases/tag/v0.2.0", http.StatusOK)
	defer srv.Close()

	vc := newTestChecker(srv.URL, "v0.1.0", false)
	vc.Start(t.Context(), 0) // 無効のため何もしない（コンテキストは保持する）
	if vc.isRunning() {
		t.Fatal("無効なのに開始した")
	}

	vc.SetEnabled(true)
	if !vc.isRunning() {
		t.Fatal("SetEnabled(true) で再開しない")
	}
	deadline := time.Now().Add(5 * time.Second)
	for !vc.UpdateAvailable() {
		if time.Now().After(deadline) {
			t.Fatal("有効化後にチェックが行われない")
		}
		time.Sleep(10 * time.Millisecond)
	}

	vc.SetEnabled(false)
	if vc.isRunning() {
		t.Error("SetEnabled(false) で停止しない")
	}
}

func TestVersionChecker_SetEnabledBeforeStartDoesNotRun(t *testing.T) {
	vc := newTestChecker("http://127.0.0.1:0", "v0.1.0", false)
	vc.SetEnabled(true)
	if vc.isRunning() {
		t.Error("Start 前の SetEnabled(true) でチェックを始めた")
	}
}

func (vc *VersionChecker) isRunning() bool {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.cancel != nil
}
