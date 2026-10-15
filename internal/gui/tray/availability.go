package tray

import "time"

// ReadyTimeout は systray の登録完了コールバック（onReady）を待つ最大時間。
// この時間内に到達しなければトレイ利用不可として扱う（NFR-37）。
const ReadyTimeout = 5 * time.Second

// HostCheck はトレイホストの存在確認。
// required はこの OS がトレイホストを必要とするか（Linux の StatusNotifierWatcher）、
// present はホストが存在するか。required が false の OS では present を見ない。
type HostCheck func() (required, present bool)

// Available はトレイ利用可否を決定する純粋関数。
//
//	readyReached  hostRequired  hostPresent  結果
//	false         *             *            false  (onReady に到達しない)
//	true          false         *            true   (macOS / Windows。ホスト確認不要)
//	true          true          true         true
//	true          true          false        false  (Linux で StatusNotifierWatcher 不在)
func Available(readyReached, hostRequired, hostPresent bool) bool {
	if !readyReached {
		return false
	}
	return !hostRequired || hostPresent
}

// Detect は起動時に 1 回だけトレイ利用可否を判定する。ready が close されるのを timeout まで待ち、
// 到達したら host でホストの存在を確認する。トレイが後から消えた場合の検知は行わない。
func Detect(ready <-chan struct{}, timeout time.Duration, host HostCheck) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ready:
	case <-timer.C:
		return Available(false, false, false)
	}
	required, present := host()
	return Available(true, required, present)
}
