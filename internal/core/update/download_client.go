package update

import (
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
