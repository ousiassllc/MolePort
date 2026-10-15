package daemon

import (
	"os"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/core/update"
)

// Services は内蔵ホスト（GUI）が Core を直接操作するための参照をまとめた構造体。
type Services struct {
	Config  core.ConfigManager
	SSH     core.SSHManager
	Forward core.ForwardManager
	Version *update.VersionChecker
}

// Info はデーモンの実行情報（ヘッダー表示用）。
type Info struct {
	Version   string
	PID       int
	StartedAt time.Time
}

// Services は Core マネージャーの参照を返す。Start 後に有効。
func (d *Daemon) Services() Services {
	return Services{
		Config:  d.cfgMgr,
		SSH:     d.sshMgr,
		Forward: d.fwdMgr,
		Version: d.versionChecker,
	}
}

// Done はデーモンのコンテキストが終了したときに close されるチャネルを返す。
// daemon.shutdown (IPC)・Shutdown()・Stop() のいずれでも close される。
// 内蔵ホストは Wait()（シグナル待ち）の代わりにこれを監視し、close されたら Stop() を呼ぶ。
func (d *Daemon) Done() <-chan struct{} {
	return d.ctx.Done()
}

// Info はバージョン・PID・起動時刻を返す。StartedAt は Start 後に有効。
func (d *Daemon) Info() Info {
	return Info{
		Version:   d.version,
		PID:       os.Getpid(),
		StartedAt: d.startedAt,
	}
}
