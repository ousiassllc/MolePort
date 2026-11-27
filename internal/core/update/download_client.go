package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// maxDownloadSize はダウンロードファイルの最大サイズ (256 MiB)。
	maxDownloadSize = 256 << 20

	// maxReleaseResponseSize はリリース API レスポンスの最大サイズ (1 MiB)。
	maxReleaseResponseSize = 1 << 20

	// downloadTimeout は 1 ファイルのダウンロード全体（接続から本文の読み取りまで）の上限。
	downloadTimeout = 10 * time.Minute

	// downloadResponseHeaderTimeout はリクエスト送信後、レスポンスヘッダーを待つ上限。
	downloadResponseHeaderTimeout = 30 * time.Second
)

// newDownloadClient はアセットのダウンロード用 HTTP クライアントを返す。
// バージョンチェック用クライアントの Timeout（5 秒）は本文の読み取りまで含むため、
// 数 MB 以上のアーカイブのダウンロードには使えない。ここでは Timeout を設定しない。
func newDownloadClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = downloadResponseHeaderTimeout
	return &http.Client{Transport: transport}
}

// downloadFile は指定 URL からファイルをダウンロードする。
func (u *Updater) downloadFile(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "MolePort/"+u.checker.currentVersion)

	resp, err := u.downloadClient.Do(req) //nolint:gosec // URL comes from GitHub API response
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
