package update

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUpdate_FullFlow(t *testing.T) {
	tmpDir := t.TempDir()
	execPath := filepath.Join(tmpDir, "moleport")
	if err := os.WriteFile(execPath, []byte("old-binary"), 0o755); err != nil { //nolint:gosec // テスト用実行可能ファイル
		t.Fatalf("write test binary: %v", err)
	}

	newContent := []byte("new-binary-v1.1.0")
	assetName := fmt.Sprintf("moleport_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	tarGzData := createTestTarGz(t, "moleport", newContent)
	checksumData := createTestChecksums(t, map[string][]byte{assetName: tarGzData})

	srv := newUpdaterTestServer(t, "v1.1.0",
		map[string]githubRelease{
			"v1.1.0": {TagName: "v1.1.0", Assets: []githubReleaseAsset{
				{Name: assetName}, {Name: "checksums.txt"},
			}},
		},
		map[string][]byte{assetName: tarGzData, "checksums.txt": checksumData},
	)
	defer srv.Close()

	u := NewUpdater(newTestChecker(srv.URL, "v1.0.0", true))
	u.execPath = execPath

	if err := u.Update(t.Context(), nil); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := os.ReadFile(execPath) //nolint:gosec // テスト用パス
	if err != nil {
		t.Fatalf("read updated binary: %v", err)
	}
	if string(got) != string(newContent) {
		t.Errorf("binary content = %q, want %q", got, newContent)
	}

	info, err := os.Stat(execPath)
	if err != nil {
		t.Fatalf("stat updated binary: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("binary permissions = %o, want %o", info.Mode().Perm(), 0o755)
	}
}

func TestUpdate_ChecksumMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	execPath := filepath.Join(tmpDir, "moleport")
	if err := os.WriteFile(execPath, []byte("old"), 0o755); err != nil { //nolint:gosec // テスト用実行可能ファイル
		t.Fatalf("write test binary: %v", err)
	}

	assetName := fmt.Sprintf("moleport_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	tarGzData := createTestTarGz(t, "moleport", []byte("new-binary"))
	wrongChecksumData := fmt.Appendf(nil, "%s  %s\n",
		"0000000000000000000000000000000000000000000000000000000000000000", assetName)

	srv := newUpdaterTestServer(t, "v1.1.0",
		map[string]githubRelease{
			"v1.1.0": {TagName: "v1.1.0", Assets: []githubReleaseAsset{
				{Name: assetName}, {Name: "checksums.txt"},
			}},
		},
		map[string][]byte{assetName: tarGzData, "checksums.txt": wrongChecksumData},
	)
	defer srv.Close()

	u := NewUpdater(newTestChecker(srv.URL, "v1.0.0", true))
	u.execPath = execPath

	err := u.Update(t.Context(), nil)
	if err == nil {
		t.Fatal("Update() should return error on checksum mismatch")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error should contain 'checksum mismatch', got: %v", err)
	}

	got, err := os.ReadFile(execPath) //nolint:gosec // テスト用パス
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if string(got) != "old" {
		t.Error("binary should not be modified on checksum mismatch")
	}
}

func TestUpdate_ProgressCallback(t *testing.T) {
	tmpDir := t.TempDir()
	execPath := filepath.Join(tmpDir, "moleport")
	if err := os.WriteFile(execPath, []byte("old"), 0o755); err != nil { //nolint:gosec // テスト用実行可能ファイル
		t.Fatalf("write test binary: %v", err)
	}

	assetName := fmt.Sprintf("moleport_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	tarGzData := createTestTarGz(t, "moleport", []byte("new"))
	checksumData := createTestChecksums(t, map[string][]byte{assetName: tarGzData})

	srv := newUpdaterTestServer(t, "v1.1.0",
		map[string]githubRelease{
			"v1.1.0": {TagName: "v1.1.0", Assets: []githubReleaseAsset{
				{Name: assetName}, {Name: "checksums.txt"},
			}},
		},
		map[string][]byte{assetName: tarGzData, "checksums.txt": checksumData},
	)
	defer srv.Close()

	u := NewUpdater(newTestChecker(srv.URL, "v1.0.0", true))
	u.execPath = execPath

	var stages []string
	if err := u.Update(t.Context(), func(stage string) {
		stages = append(stages, stage)
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	expected := []string{"checking", "downloading", "verifying", "extracting", "replacing"}
	if len(stages) != len(expected) {
		t.Fatalf("stages = %v, want %v", stages, expected)
	}
	for i, s := range expected {
		if stages[i] != s {
			t.Errorf("stages[%d] = %q, want %q", i, stages[i], s)
		}
	}
}
