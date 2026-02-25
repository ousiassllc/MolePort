package daemon

import (
	"log/slog"
	"os"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// startEventRouting は SSH/Forward イベントをブローカーにルーティングするゴルーチンを開始する。
func (d *Daemon) startEventRouting() {
	sshEvents := d.sshMgr.Subscribe()
	fwdEvents := d.fwdMgr.Subscribe()

	d.wg.Add(2)
	go func() {
		defer d.wg.Done()
		for evt := range sshEvents {
			d.broker.HandleSSHEvent(evt)
		}
	}()

	go func() {
		defer d.wg.Done()
		for evt := range fwdEvents {
			d.broker.HandleForwardEvent(evt)
		}
	}()
}

// restoreState は前回の状態を復元する。auto_restore が有効な場合のみ。
func (d *Daemon) restoreState() {
	cfg := d.cfgMgr.GetConfig()
	if !cfg.Session.AutoRestore {
		return
	}

	state, err := d.cfgMgr.LoadState()
	if err != nil {
		slog.Debug("no state to restore", "error", err)
		return
	}

	for _, rule := range state.ActiveForwards {
		if err := d.fwdMgr.StartForward(rule.Name); err != nil {
			slog.Warn("failed to restore forward", "rule", rule.Name, "error", err)
		}
	}
}

// saveState はアクティブなフォワード状態を保存する。
func (d *Daemon) saveState() {
	sessions := d.fwdMgr.GetAllSessions()
	var activeRules []core.ForwardRule
	for _, s := range sessions {
		if s.Status == core.Active {
			activeRules = append(activeRules, s.Rule)
		}
	}

	state := &core.State{
		LastUpdated:    time.Now(),
		ActiveForwards: activeRules,
	}

	if err := d.cfgMgr.SaveState(state); err != nil {
		slog.Warn("failed to save state", "error", err)
	}
}

// --- DaemonInfo インターフェースの実装 ---

// Status はデーモンの現在の状態を返す。
func (d *Daemon) Status() protocol.DaemonStatusResult {
	sessions := d.fwdMgr.GetAllSessions()
	activeForwards := 0
	for _, s := range sessions {
		if s.Status == core.Active {
			activeForwards++
		}
	}

	// SSH 接続数はキャッシュ済みホスト一覧から計算する（再解析の副作用なし）
	activeSSH := 0
	for _, h := range d.sshMgr.GetHosts() {
		if h.State == core.Connected {
			activeSSH++
		}
	}

	connectedClients := 0
	if d.server != nil {
		connectedClients = d.server.ConnectedClients()
	}

	return protocol.DaemonStatusResult{
		PID:                  os.Getpid(),
		StartedAt:            d.startedAt.Format(time.RFC3339),
		Uptime:               time.Since(d.startedAt).Truncate(time.Second).String(),
		ConnectedClients:     connectedClients,
		ActiveSSHConnections: activeSSH,
		ActiveForwards:       activeForwards,
	}
}
