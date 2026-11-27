package update

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// VersionChecker の短い Timeout を超える時間がかかるダウンロードでも成功する。
func TestDownloadFile_SucceedsBeyondCheckerTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "part1-")
		w.(http.Flusher).Flush()
		select {
		case <-release:
			_, _ = io.WriteString(w, "part2")
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	vc := New("v1.0.0", true, time.Hour)
	vc.httpClient.Timeout = 50 * time.Millisecond
	u := NewUpdater(vc)

	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		data, err := u.downloadFile(t.Context(), srv.URL)
		done <- result{data, err}
	}()

	// チェッカー用クライアントでは同じ応答の本文読み取りがタイムアウトすることを確認し、
	// ダウンロードが Timeout を超える時間をかけていることを保証してから応答を完了させる。
	resp, err := vc.httpClient.Get(srv.URL)
	if err != nil {
		t.Fatalf("checker client Get() error = %v", err)
	}
	if _, err := io.ReadAll(resp.Body); err == nil {
		t.Fatal("checker client should time out while reading the slow body")
	}
	_ = resp.Body.Close()
	close(release)

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("downloadFile() error = %v", r.err)
		}
		if string(r.data) != "part1-part2" {
			t.Errorf("downloadFile() = %q, want %q", r.data, "part1-part2")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("downloadFile() did not finish")
	}
}

func TestNewUpdater_DownloadClientHasNoTotalTimeout(t *testing.T) {
	u := NewUpdater(New("v1.0.0", true, time.Hour))
	if u.downloadClient == u.httpClient {
		t.Error("downloadClient must not share the version check client")
	}
	if u.downloadClient.Timeout != 0 {
		t.Errorf("downloadClient.Timeout = %v, want 0", u.downloadClient.Timeout)
	}
	if u.httpClient.Timeout == 0 {
		t.Error("release lookup client should keep its short timeout")
	}
}
