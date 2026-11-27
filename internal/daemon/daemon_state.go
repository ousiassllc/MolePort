package daemon

import (
	"log/slog"
	"os"
	"slices"
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
			case core.SSHEventDisconnected:
				// 再接続待ち中にユーザーが切断した場合、再接続中フラグを残さない
				// （通常の切断検知では Disconnected の後に Reconnecting が届く）
				delete(reconnecting, evt.HostName)
			case core.SSHEventConnected:
				if reconnecting[evt.HostName] {
					delete(reconnecting, evt.HostName)
					results := d.fwdMgr.RestoreForwards(evt.HostName)
					d.logRestoreSummary(evt.HostName, results)
				}
			case core.SSHEventError, core.SSHEventPendingAuth:
				// 再接続の失敗、または認証失敗による再接続の打ち切り（手動 connect 待ち）
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

// startBackgroundRestore はセッション復元と auto_connect の接続を d.wg 管理下の
// goroutine で実行する。SSH ダイヤルに時間がかかっても Start をブロックしない（NFR-01）。
func (d *Daemon) startBackgroundRestore() {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.restoreState()
		d.autoStartForwards()
	}()
}

// shuttingDown はデーモンの停止が始まっているかを返す。
func (d *Daemon) shuttingDown() bool {
	return d.ctx != nil && d.ctx.Err() != nil
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

	d.setPendingRestore(state.ActiveForwards)
	for _, rule := range state.ActiveForwards {
		if d.shuttingDown() {
			return
		}
		err := d.fwdMgr.StartForward(rule.Name, nil)
		if err != nil && d.shuttingDown() {
			// 停止による中断は失敗ではない。未復元として残し、saveState が state.yaml に引き継ぐ。
			return
		}
		if err != nil {
			slog.Warn("failed to restore forward", "rule", rule.Name, "error", err)
		}
		d.markRestoreProcessed(rule.Name)
	}
}

// setPendingRestore は復元待ちのルール一覧を記録する。
func (d *Daemon) setPendingRestore(rules []core.ForwardRule) {
	d.restoreMu.Lock()
	defer d.restoreMu.Unlock()
	d.pendingRestore = append([]core.ForwardRule(nil), rules...)
}

// markRestoreProcessed は復元の処理が終わったルールを復元待ちから外す。
func (d *Daemon) markRestoreProcessed(name string) {
	d.restoreMu.Lock()
	defer d.restoreMu.Unlock()
	d.pendingRestore = slices.DeleteFunc(d.pendingRestore, func(r core.ForwardRule) bool { return r.Name == name })
}

// pendingRestoreSnapshot は復元待ちのルールのコピーを返す。
func (d *Daemon) pendingRestoreSnapshot() []core.ForwardRule {
	d.restoreMu.Lock()
	defer d.restoreMu.Unlock()
	return append([]core.ForwardRule(nil), d.pendingRestore...)
}

// autoStartForwards は config.yaml で auto_connect が有効なフォワードルールを自動開始する。
// restoreState() で既に開始済みのルールはスキップする。
func (d *Daemon) autoStartForwards() {
	cfg := d.cfgMgr.GetConfig()

	var started, skipped, failed int
	for _, rule := range cfg.Forwards {
		if d.shuttingDown() {
			break
		}
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
// 非同期の復元が終わる前に停止した場合に備え、未復元のルールも保存対象に含める（NFR-16 / UC-9）。
func (d *Daemon) saveState() {
	// 復元の完了でルールが復元待ちから外れる前に Active になっているため、
	// 復元待ちを先に取得してからセッションを読めば、どちらにも載らないルールは出ない。
	pending := d.pendingRestoreSnapshot()
	sessions := d.fwdMgr.GetAllSessions()
	var activeRules []core.ForwardRule
	saved := make(map[string]bool)
	for _, s := range sessions {
		if s.Status == core.Active {
			activeRules = append(activeRules, s.Rule)
			saved[s.Rule.Name] = true
		}
	}
	for _, rule := range pending {
		if !saved[rule.Name] {
			activeRules = append(activeRules, rule)
		}
	}

	state := &core.State{
		LastUpdated:    time.Now(),
		ActiveForwards: activeRules,
	}

	if err := d.cfgMgr.SaveState(state); err != nil {
		slog.Warn("failed to save state, retrying", "error", err)
		time.Sleep(100 * time.Millisecond)
		if err := d.cfgMgr.SaveState(state); err != nil {
			slog.Warn("failed to save state after retry", "error", err)
		}
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
		Version:              d.version,
		PID:                  os.Getpid(),
		StartedAt:            d.startedAt.Format(time.RFC3339),
		Uptime:               time.Since(d.startedAt).Truncate(time.Second).String(),
		ConnectedClients:     connectedClients,
		ActiveSSHConnections: activeSSH,
		ActiveForwards:       activeForwards,
		Warnings:             d.warnings,
	}
}
