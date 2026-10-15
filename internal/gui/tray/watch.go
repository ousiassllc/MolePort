package tray

import (
	"context"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
)

// ReconcileInterval は Core のイベントを伴わない変更（CLI/TUI によるルールの追加・削除）を拾う周期。
const ReconcileInterval = 5 * time.Second

// Watch は SSH / 転送イベントと周期ティッカーのたびに trigger を呼ぶ。ctx のキャンセルで戻る。
// ウィンドウ非表示中は EventBridge が通知を止める（NFR-33）ため、トレイは独自に購読して追従する。
// trigger はデバウンスされる前提で、ブロックしてはならない。
func Watch(ctx context.Context, ssh core.SSHManager, fwd core.ForwardManager, interval time.Duration, trigger func()) {
	sshCh := ssh.Subscribe()
	fwdCh := fwd.Subscribe()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-sshCh:
			if !ok {
				sshCh = nil
				continue
			}
		case _, ok := <-fwdCh:
			if !ok {
				fwdCh = nil
				continue
			}
		case <-ticker.C:
		}
		trigger()
	}
}
