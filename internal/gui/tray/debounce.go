package tray

import (
	"sync"
	"time"
)

// RefreshDelay はトレイメニュー更新のデバウンス間隔（docs/architecture/gui.md「システムトレイ」）。
const RefreshDelay = 500 * time.Millisecond

// Debouncer は短時間に重なった Trigger を、最初の Trigger から delay 後の 1 回の fn 呼び出しにまとめる。
// 連続するイベントでも更新が無限に先送りされない。fn は直列に実行される。
type Debouncer struct {
	delay time.Duration
	fn    func()

	mu      sync.Mutex
	pending *time.Timer
	stopped bool
}

// NewDebouncer は Debouncer を返す。
func NewDebouncer(delay time.Duration, fn func()) *Debouncer {
	return &Debouncer{delay: delay, fn: fn}
}

// Trigger は fn の実行を予約する。すでに予約済みなら何もしない。Stop 後は何もしない。
func (d *Debouncer) Trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped || d.pending != nil {
		return
	}
	d.pending = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		d.pending = nil
		stopped := d.stopped
		d.mu.Unlock()
		if !stopped {
			d.fn()
		}
	})
}

// Stop は予約を取り消し、以後の Trigger を無視する。
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopped = true
	if d.pending != nil {
		d.pending.Stop()
		d.pending = nil
	}
}
