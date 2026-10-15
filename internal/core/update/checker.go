// Package update は GitHub リリースを使った自動アップデートチェック機能を提供する。
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"

	"github.com/ousiassllc/moleport/internal/core"
)

// githubReleaseAsset は GitHub リリースのアセット情報を表す。
type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// githubRelease は GitHub API のリリースレスポンスの必要フィールドを表す。
type githubRelease struct {
	TagName string               `json:"tag_name"`
	HTMLURL string               `json:"html_url"`
	Assets  []githubReleaseAsset `json:"assets"`
}

// VersionChecker は GitHub リリースを定期的にチェックし、
// 新しいバージョンが利用可能かどうかを判定する。
type VersionChecker struct {
	currentVersion string
	repoOwner      string
	repoName       string
	httpClient     *http.Client
	apiBase        string // テスト用。デフォルトは "https://api.github.com"
	interval       time.Duration
	enabled        bool // mu で保護する（SetEnabled で実行中に変わる）
	cache          *core.VersionCheckResult
	mu             sync.RWMutex
	parent         context.Context // Start に渡されたコンテキスト。SetEnabled(true) での再開に使う
	ctx            context.Context
	cancel         context.CancelFunc
}

// New は VersionChecker を生成する。
func New(currentVersion string, enabled bool, interval time.Duration) *VersionChecker {
	return &VersionChecker{
		currentVersion: currentVersion,
		repoOwner:      "ousiassllc",
		repoName:       "moleport",
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		apiBase:  "https://api.github.com",
		interval: interval,
		enabled:  enabled,
	}
}

// SetAPIBase はテスト用に API ベース URL を差し替える。
func (vc *VersionChecker) SetAPIBase(base string) {
	vc.apiBase = base
}

// Start はバックグラウンドゴルーチンで定期的なバージョンチェックを開始する。
// enabled が false またはバージョンが "dev" の場合は何もしない（ctx は SetEnabled(true) での再開用に保持する）。
// initialDelay 後に最初のチェックを行い、以後 interval ごとにチェックする。
func (vc *VersionChecker) Start(ctx context.Context, initialDelay time.Duration) {
	vc.mu.Lock()
	vc.parent = ctx
	if !vc.enabled || vc.currentVersion == "dev" || vc.cancel != nil {
		vc.mu.Unlock()
		return // 無効、または二重起動防止
	}
	vc.ctx, vc.cancel = context.WithCancel(ctx)
	runCtx := vc.ctx
	vc.mu.Unlock()

	go vc.loop(runCtx, initialDelay)
}

// SetEnabled はチェックの有効/無効を実行中に切り替える（GUI の設定画面、UC-34）。
// 無効にするとバックグラウンドチェックを停止し、有効にすると Start 済みであれば直ちにチェックを再開する。
func (vc *VersionChecker) SetEnabled(enabled bool) {
	vc.mu.Lock()
	vc.enabled = enabled
	parent := vc.parent
	vc.mu.Unlock()

	if !enabled {
		vc.Stop()
		return
	}
	if parent != nil {
		vc.Start(parent, 0)
	}
}

// active はチェックが有効（enabled かつ "dev" でない）かを返す。
func (vc *VersionChecker) active() bool {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.enabled && vc.currentVersion != "dev"
}

// Cached はキャッシュ済みのチェック結果を返す。ネットワークアクセスは行わない。
// 無効・"dev"・未チェックの場合は ok=false。
func (vc *VersionChecker) Cached() (core.VersionCheckResult, bool) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	if !vc.enabled || vc.currentVersion == "dev" || vc.cache == nil {
		return core.VersionCheckResult{}, false
	}
	return *vc.cache, true
}

// Stop はバックグラウンドチェックを停止する。冪等。
func (vc *VersionChecker) Stop() {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	if vc.cancel != nil {
		vc.cancel()
		vc.cancel = nil
	}
}

// LatestVersion はキャッシュされた結果を返す。
// キャッシュがない場合は即時チェックを行う。
// disabled または "dev" の場合は nil, nil を返す。
func (vc *VersionChecker) LatestVersion(ctx context.Context) (*core.VersionCheckResult, error) {
	if !vc.active() {
		return nil, nil
	}

	vc.mu.RLock()
	if vc.cache != nil {
		result := *vc.cache
		vc.mu.RUnlock()
		return &result, nil
	}
	vc.mu.RUnlock()

	if err := vc.check(ctx); err != nil {
		return nil, err
	}

	vc.mu.RLock()
	defer vc.mu.RUnlock()
	if vc.cache != nil {
		result := *vc.cache
		return &result, nil
	}
	return nil, nil
}

// UpdateAvailable はアップデートが利用可能かどうかを返す。
// プロダクションコードでは GetResult() で取得した結果の UpdateAvailable フィールドを使用する。
// このメソッドは主にテストで使用される。
func (vc *VersionChecker) UpdateAvailable() bool {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.cache != nil && vc.cache.UpdateAvailable
}

// loop はバックグラウンドで定期的にバージョンチェックを行う。
func (vc *VersionChecker) loop(ctx context.Context, initialDelay time.Duration) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(initialDelay):
	}

	if err := vc.check(ctx); err != nil {
		slog.Warn("version check failed", "error", err)
	}

	ticker := time.NewTicker(vc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := vc.check(ctx); err != nil {
				slog.Warn("version check failed", "error", err)
			}
		}
	}
}

// check は GitHub API から最新リリースを取得し、キャッシュを更新する。
func (vc *VersionChecker) check(ctx context.Context) error {
	release, err := vc.fetchLatest(ctx)
	if err != nil {
		return err
	}

	newer := isNewerVersion(vc.currentVersion, release.TagName)

	result := &core.VersionCheckResult{
		LatestVersion:   release.TagName,
		ReleaseURL:      release.HTMLURL,
		CheckedAt:       time.Now(),
		UpdateAvailable: newer,
	}

	vc.mu.Lock()
	vc.cache = result
	vc.mu.Unlock()

	if newer {
		slog.Info("new version available", "current", vc.currentVersion, "latest", release.TagName)
	}
	return nil
}

// fetchLatest は GitHub API から最新リリース情報を取得する。
func (vc *VersionChecker) fetchLatest(ctx context.Context) (*githubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", vc.apiBase, vc.repoOwner, vc.repoName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "MolePort/"+vc.currentVersion)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := vc.httpClient.Do(req) //nolint:gosec // URL is built from hardcoded repo constants
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
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

// isNewerVersion は latest が current より新しいかどうかを判定する。
// semver 形式（"v" プレフィックス付き）で比較する。
func isNewerVersion(current, latest string) bool {
	c := ensureVPrefix(current)
	l := ensureVPrefix(latest)

	if !semver.IsValid(c) || !semver.IsValid(l) {
		return false
	}
	return semver.Compare(l, c) > 0
}

// ensureVPrefix は "v" プレフィックスがない場合に付加する。
func ensureVPrefix(version string) string {
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}
