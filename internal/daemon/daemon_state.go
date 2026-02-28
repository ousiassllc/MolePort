package daemon

import (
	"log/slog"
	"os"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// startEventRouting は SSH/Forward イベントをブローカーにルーティングするゴルーチンを開始する。
// SSH 再接続イベントを検知してフォワード復元をトリガーする。
func (d *Daemon) startEventRouting() {
	sshEvents := d.sshMgr.Subscribe()
	fwdEvents := d.fwdMgr.Subscribe()

	d.wg.Add(2)
	go func() {
		defer d.wg.Done()
		reconnecting := make(map[string]bool)
		for evt := range sshEvents {
			d.broker.HandleSSHEvent(evt)
			switch evt.Type {
			case core.SSHEventReconnecting:
				reconnecting[evt.HostName] = true
				d.fwdMgr.MarkReconnecting(evt.HostName)
			case core.SSHEventConnected:
				if reconnecting[evt.HostName] {
					delete(reconnecting, evt.HostName)
					results := d.fwdMgr.RestoreForwards(evt.HostName)
					d.logRestoreSummary(evt.HostName, results)
				}
			case core.SSHEventError:
				if reconnecting[evt.HostName] {
					delete(reconnecting, evt.HostName)
					d.fwdMgr.FailReconnecting(evt.HostName)
				}
			}
		}
	}()

	go func() {
		defer d.wg.Done()
		for evt := range fwdEvents {
			d.broker.HandleForwardEvent(evt)
		}
	}()
}

// logRestoreSummary はフォワード復元結果のサマリーをログ出力する。
func (d *Daemon) logRestoreSummary(hostName string, results []core.ForwardRestoreResult) {
	if len(results) == 0 {
		return
	}
	succeeded, failed := 0, 0
	for _, r := range results {
		if r.OK {
			succeeded++
		} else {
			failed++
			slog.Warn("forward restore failed", "host", hostName, "rule", r.RuleName, "error", r.Error)
		}
	}
	slog.Info("forward restore summary", "host", hostName, "total", len(results), "succeeded", succeeded, "failed", failed)
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
		if err := d.fwdMgr.StartForward(rule.Name, nil); err != nil {
			slog.Warn("failed to restore forward", "rule", rule.Name, "error", err)
		}
	}
}

// autoStartForwards は config.yaml で auto_connect が有効なフォワードルールを自動開始する。
// restoreState() で既に開始済みのルールはスキップする。
func (d *Daemon) autoStartForwards() {
	cfg := d.cfgMgr.GetConfig()

	var started, skipped, failed int
	for _, rule := range cfg.Forwards {
		if !rule.AutoConnect {
			continue
		}
		// restoreState() で既にアクティブなルールはスキップ
		if s, err := d.fwdMgr.GetSession(rule.Name); err == nil && s.Status == core.Active {
			skipped++
			continue
		}
		// cb=nil: daemon 起動時は対話的認証が不可のため、鍵認証/エージェントのみで接続を試みる
		if err := d.fwdMgr.StartForward(rule.Name, nil); err != nil {
			slog.Warn("auto-start forward failed", "rule", rule.Name, "error", err)
			failed++
		} else {
			started++
		}
	}

	if started+failed+skipped > 0 {
		slog.Info("auto-start forwards summary", "started", started, "skipped", skipped, "failed", failed)
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
