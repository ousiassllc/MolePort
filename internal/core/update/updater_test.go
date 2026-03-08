package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

// createTestTarGz はテスト用の tar.gz アーカイブを作成する。
func createTestTarGz(t *testing.T, filename string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{Name: filename, Mode: 0o755, Size: int64(len(content))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write tar content: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buf.Bytes()
}

// createTestChecksums はテスト用の checksums.txt を作成する。
func createTestChecksums(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var lines []string
	for name, data := range entries {
		h := sha256.Sum256(data)
		lines = append(lines, fmt.Sprintf("%x  %s", h, name))
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

// newUpdaterTestServer は自己参照型のテストサーバーを作成する。
// リリースアセットの BrowserDownloadURL は自動的にサーバー URL に解決される。
func newUpdaterTestServer(t *testing.T, latestTag string, releases map[string]githubRelease, downloads map[string][]byte) *httptest.Server {
	t.Helper()
	var serverURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/ousiassllc/moleport/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(githubRelease{
				TagName: latestTag,
				HTMLURL: "https://github.com/ousiassllc/moleport/releases/tag/" + latestTag,
			})
		case strings.HasPrefix(r.URL.Path, "/repos/ousiassllc/moleport/releases/tags/"):
			tag := strings.TrimPrefix(r.URL.Path, "/repos/ousiassllc/moleport/releases/tags/")
			rel, ok := releases[tag]
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			// アセット URL をサーバー URL に解決
			resolved := rel
			resolved.Assets = make([]githubReleaseAsset, len(rel.Assets))
			for i, a := range rel.Assets {
				resolved.Assets[i] = a
				if a.BrowserDownloadURL == "" {
					resolved.Assets[i].BrowserDownloadURL = serverURL + "/download/" + a.Name
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resolved)
		case strings.HasPrefix(r.URL.Path, "/download/"):
			filename := strings.TrimPrefix(r.URL.Path, "/download/")
			data, ok := downloads[filename]
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(data)
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	serverURL = srv.URL
	return srv
}

func TestNewUpdater(t *testing.T) {
	vc := New("v1.0.0", true, time.Hour)
	vc.apiBase = "https://test.example.com"
	u := NewUpdater(vc)

	if u.checker != vc {
		t.Error("checker should be the same VersionChecker")
	}
	if u.httpClient != vc.httpClient {
		t.Error("httpClient should be shared with VersionChecker")
	}
	if u.repoOwner != "ousiassllc" {
		t.Errorf("repoOwner = %q, want %q", u.repoOwner, "ousiassllc")
	}
	if u.repoName != "moleport" {
		t.Errorf("repoName = %q, want %q", u.repoName, "moleport")
	}
	if u.apiBase != "https://test.example.com" {
		t.Errorf("apiBase = %q, want %q", u.apiBase, "https://test.example.com")
	}
}

func TestFindAsset_Success(t *testing.T) {
	assetName := fmt.Sprintf("moleport_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	srv := newUpdaterTestServer(t, "v1.1.0", map[string]githubRelease{
		"v1.1.0": {
			TagName: "v1.1.0",
			Assets: []githubReleaseAsset{
				{Name: assetName, BrowserDownloadURL: "https://example.com/" + assetName},
				{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
			},
		},
	}, nil)
	defer srv.Close()

	u := NewUpdater(newTestChecker(srv.URL, "v1.0.0", true))
	assetURL, checksumURL, err := u.FindAsset(t.Context(), "v1.1.0")
	if err != nil {
		t.Fatalf("FindAsset() error = %v", err)
	}
	if assetURL != "https://example.com/"+assetName {
		t.Errorf("assetURL = %q, want %q", assetURL, "https://example.com/"+assetName)
	}
	if checksumURL != "https://example.com/checksums.txt" {
		t.Errorf("checksumURL = %q, want %q", checksumURL, "https://example.com/checksums.txt")
	}
}

func TestFindAsset_MissingAsset(t *testing.T) {
	srv := newUpdaterTestServer(t, "v1.1.0", map[string]githubRelease{
		"v1.1.0": {
			TagName: "v1.1.0",
			Assets: []githubReleaseAsset{
				{Name: "moleport_other_os.tar.gz"},
				{Name: "checksums.txt"},
			},
		},
	}, nil)
	defer srv.Close()

	u := NewUpdater(newTestChecker(srv.URL, "v1.0.0", true))
	_, _, err := u.FindAsset(t.Context(), "v1.1.0")
	if err == nil {
		t.Fatal("FindAsset() should return error for missing asset")
	}
}

func TestFindAsset_MissingChecksums(t *testing.T) {
	assetName := fmt.Sprintf("moleport_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	srv := newUpdaterTestServer(t, "v1.1.0", map[string]githubRelease{
		"v1.1.0": {
			TagName: "v1.1.0",
			Assets:  []githubReleaseAsset{{Name: assetName}},
		},
	}, nil)
	defer srv.Close()

	u := NewUpdater(newTestChecker(srv.URL, "v1.0.0", true))
	_, _, err := u.FindAsset(t.Context(), "v1.1.0")
	if err == nil {
		t.Fatal("FindAsset() should return error for missing checksums.txt")
	}
}

func TestUpdate_DevBuild(t *testing.T) {
	err := NewUpdater(New("dev", true, time.Hour)).Update(t.Context(), nil)
	if err != ErrDevBuild {
		t.Errorf("Update() error = %v, want %v", err, ErrDevBuild)
	}
}

func TestUpdate_AlreadyLatest(t *testing.T) {
	srv := newUpdaterTestServer(t, "v1.0.0", nil, nil)
	defer srv.Close()

	u := NewUpdater(newTestChecker(srv.URL, "v1.0.0", true))
	err := u.Update(t.Context(), nil)
	if err != ErrAlreadyLatest {
		t.Errorf("Update() error = %v, want %v", err, ErrAlreadyLatest)
	}
}

func TestVerifyChecksum(t *testing.T) {
	data := []byte("test binary data")
	h := sha256.Sum256(data)
	correctHash := fmt.Sprintf("%x", h)

	t.Run("correct", func(t *testing.T) {
		checksums := fmt.Appendf(nil, "%s  moleport.tar.gz\n", correctHash)
		if err := verifyChecksum(data, checksums, "moleport.tar.gz"); err != nil {
			t.Errorf("verifyChecksum() error = %v", err)
		}
	})
	t.Run("wrong", func(t *testing.T) {
		checksums := []byte("0000000000000000000000000000000000000000000000000000000000000000  moleport.tar.gz\n")
		err := verifyChecksum(data, checksums, "moleport.tar.gz")
		if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
			t.Errorf("expected checksum mismatch error, got: %v", err)
		}
	})
	t.Run("missing filename", func(t *testing.T) {
		checksums := fmt.Appendf(nil, "%s  other_file.tar.gz\n", correctHash)
		err := verifyChecksum(data, checksums, "moleport.tar.gz")
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected not found error, got: %v", err)
		}
	})
}

func TestExtractBinary(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		content := []byte("binary content here")
		got, err := extractBinary(createTestTarGz(t, "moleport", content), "moleport")
		if err != nil {
			t.Fatalf("extractBinary() error = %v", err)
		}
		if string(got) != string(content) {
			t.Errorf("extractBinary() = %q, want %q", got, content)
		}
	})
	t.Run("missing", func(t *testing.T) {
		_, err := extractBinary(createTestTarGz(t, "other", []byte("x")), "moleport")
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected not found error, got: %v", err)
		}
	})
}
