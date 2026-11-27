package daemon

import (
	"log/slog"
	"time"
)

// defaultShutdownTimeout は切断処理（NFR-16）の上限時間。
const defaultShutdownTimeout = 5 * time.Second

// closeComponents はフォワード停止・SSH クローズ・バックグラウンド goroutine の終了待ちを行う。
// NFR-16: 切断処理は shutdownTimeout 以内に完了させ、超過した場合は警告ログを出して
// 後続のソケット・PID ファイルの解放に進む（強制切断）。
func (d *Daemon) closeComponents() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := d.fwdMgr.StopAllForwards(); err != nil {
			slog.Warn("failed to stop all forwards", "error", err)
		}
		d.fwdMgr.Close()
		d.sshMgr.Close()

		// イベントルーティング・復元ゴルーチンの終了を待つ
		d.wg.Wait()
	}()

	timeout := d.shutdownTimeout
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
	case <-timer.C:
		slog.Warn("shutdown timed out; forcing close", "timeout", timeout)
	}
}
