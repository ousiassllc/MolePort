package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// エラー定数。
var (
	ErrDevBuild      = errors.New("cannot update dev build")
	ErrAlreadyLatest = errors.New("already running the latest version")
)

const (
	// maxDownloadSize はダウンロードファイルの最大サイズ (256 MiB)。
	maxDownloadSize = 256 << 20

	// maxReleaseResponseSize はリリース API レスポンスの最大サイズ (1 MiB)。
	maxReleaseResponseSize = 1 << 20
)

// Updater はセルフアップデートを実行する。
type Updater struct {
	checker    *VersionChecker
	httpClient *http.Client
	repoOwner  string
	repoName   string
	apiBase    string
	execPath   string // テスト用。空の場合は os.Executable() を使用
}

// NewUpdater は VersionChecker から Updater を生成する。
func NewUpdater(checker *VersionChecker) *Updater {
	return &Updater{
		checker:    checker,
		httpClient: checker.httpClient,
		repoOwner:  checker.repoOwner,
		repoName:   checker.repoName,
		apiBase:    checker.apiBase,
	}
}

// FindAsset は指定バージョンのリリースからアセット URL を検索する。
// tar.gz アセットと checksums.txt の URL を返す。
func (u *Updater) FindAsset(ctx context.Context, version string) (assetURL, checksumURL string, err error) {
	tag := ensureVPrefix(version)
	release, err := u.fetchRelease(ctx, tag)
	if err != nil {
		return "", "", err
	}

	expectedName := fmt.Sprintf("moleport_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)

	for _, asset := range release.Assets {
		switch asset.Name {
		case expectedName:
			assetURL = asset.BrowserDownloadURL
		case "checksums.txt":
			checksumURL = asset.BrowserDownloadURL
		}
	}

	if assetURL == "" {
		return "", "", fmt.Errorf("asset %s not found in release %s", expectedName, tag)
	}
	if checksumURL == "" {
		return "", "", fmt.Errorf("checksums.txt not found in release %s", tag)
	}
	return assetURL, checksumURL, nil
}

// Update はセルフアップデートの全フローを実行する。
// progress コールバックが非 nil の場合、各段階で呼び出される。
func (u *Updater) Update(ctx context.Context, progress func(stage string)) error {
	if progress == nil {
		progress = func(string) {}
	}

	// 1. dev ビルドチェック
	if u.checker.currentVersion == "dev" {
		return ErrDevBuild
	}

	// 2. 最新バージョンを取得
	progress("checking")
	result, err := u.checker.LatestVersion(ctx)
	if err != nil {
		return fmt.Errorf("check latest version: %w", err)
	}
	if result == nil || !result.UpdateAvailable {
		return ErrAlreadyLatest
	}

	// 3. アセット URL を取得
	assetURL, checksumURL, err := u.FindAsset(ctx, result.LatestVersion)
	if err != nil {
		return fmt.Errorf("find asset: %w", err)
	}

	// 4. tar.gz をダウンロード
	progress("downloading")
	tarGzData, err := u.downloadFile(ctx, assetURL)
	if err != nil {
		return fmt.Errorf("download asset: %w", err)
	}

	// 5. checksums.txt をダウンロード
	checksumData, err := u.downloadFile(ctx, checksumURL)
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}

	// 6. チェックサム検証
	progress("verifying")
	assetFilename := fmt.Sprintf("moleport_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	if err := verifyChecksum(tarGzData, checksumData, assetFilename); err != nil {
		return fmt.Errorf("verify checksum: %w", err)
	}

	// 7. バイナリを展開
	progress("extracting")
	binaryData, err := extractBinary(tarGzData, "moleport")
	if err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}

	// 8-10. バイナリを置換
	progress("replacing")
	execPath := u.execPath
	if execPath == "" {
		execPath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("get executable path: %w", err)
		}
	}

	if err := replaceBinary(execPath, binaryData); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}

	return nil
}

// fetchRelease は指定タグのリリース情報を GitHub API から取得する。
func (u *Updater) fetchRelease(ctx context.Context, tag string) (*githubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", u.apiBase, u.repoOwner, u.repoName, tag)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "MolePort/"+u.checker.currentVersion)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := u.httpClient.Do(req) //nolint:gosec // URL is built from hardcoded repo constants
	if err != nil {
		return nil, fmt.Errorf("fetch release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxReleaseResponseSize)).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &release, nil
}

// downloadFile は指定 URL からファイルをダウンロードする。
func (u *Updater) downloadFile(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "MolePort/"+u.checker.currentVersion)

	resp, err := u.httpClient.Do(req) //nolint:gosec // URL comes from GitHub API response
	if err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadSize+1))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if len(data) > maxDownloadSize {
		return nil, fmt.Errorf("download exceeds maximum size (%d bytes)", maxDownloadSize)
	}
	return data, nil
}

// verifyChecksum は checksums.txt の内容とデータの SHA-256 ハッシュを照合する。
// checksums.txt のフォーマット: "<sha256>  <filename>"
func verifyChecksum(data, checksumData []byte, filename string) error {
	h := sha256.Sum256(data)
	actual := hex.EncodeToString(h[:])

	for line := range strings.SplitSeq(string(checksumData), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// フォーマット: "<sha256>  <filename>" (2スペース区切り)
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.TrimSpace(parts[1]) == filename {
			expected := strings.TrimSpace(parts[0])
			if actual != expected {
				return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", filename, expected, actual)
			}
			return nil
		}
	}
	return fmt.Errorf("checksum for %s not found in checksums.txt", filename)
}

// extractBinary は tar.gz アーカイブから指定名のファイルを展開する。
func extractBinary(tarGzData []byte, binaryName string) ([]byte, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(tarGzData))
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}

		// ファイル名のベース部分で比較（パスにディレクトリが含まれる場合に対応）
		if filepath.Base(header.Name) == binaryName && header.Typeflag == tar.TypeReg {
			data, err := io.ReadAll(io.LimitReader(tr, maxDownloadSize+1))
			if err != nil {
				return nil, fmt.Errorf("read file from tar: %w", err)
			}
			if len(data) > maxDownloadSize {
				return nil, fmt.Errorf("extracted file exceeds maximum size (%d bytes)", maxDownloadSize)
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("file %s not found in archive", binaryName)
}

// replaceBinary は実行ファイルをアトミックに置換する。
func replaceBinary(execPath string, newBinary []byte) error {
	dir := filepath.Dir(execPath)
	tmpPath := filepath.Join(dir, ".moleport.update.tmp")

	// 元のファイルのパーミッションを取得
	info, err := os.Stat(execPath)
	if err != nil {
		return fmt.Errorf("stat executable: %w", err)
	}

	// 一時ファイルに書き込み
	if err := os.WriteFile(tmpPath, newBinary, info.Mode()); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}

	// アトミックにリネーム
	if err := os.Rename(tmpPath, execPath); err != nil {
		// リネーム失敗時は一時ファイルを削除
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
