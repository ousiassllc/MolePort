package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/core/forward"
	"github.com/ousiassllc/moleport/internal/core/ssh"
	"github.com/ousiassllc/moleport/internal/core/update"
	"github.com/ousiassllc/moleport/internal/infra"
	"github.com/ousiassllc/moleport/internal/infra/sshconfig"
	"github.com/ousiassllc/moleport/internal/infra/yamlstore"
	"github.com/ousiassllc/moleport/internal/ipc"
	ipchandler "github.com/ousiassllc/moleport/internal/ipc/handler"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// LogConfig はデーモンのログ設定を保持する。
type LogConfig struct {
	Path  string
	Level string
}

// ResolveLogConfig は設定ファイルからログファイルのパスとレベルを解決する。
// 設定の読み込みに失敗した場合はデフォルトの設定を使用する。
func ResolveLogConfig(configDir string) LogConfig {
	store := yamlstore.NewYAMLStore()
	cfgMgr := core.NewConfigManager(store, configDir)
	cfg, err := cfgMgr.LoadConfig()
	if err != nil {
		c := core.DefaultConfig()
		cfg = &c
	}
	logPath := cfg.Log.File
	if expanded, err := infra.ExpandTilde(logPath); err == nil {
		logPath = expanded
	}
	return LogConfig{Path: logPath, Level: cfg.Log.Level}
}

// SocketPath はデーモンの Unix ソケットパスを返す。
func SocketPath(configDir string) string {
	return filepath.Join(configDir, "moleport.sock")
}

// PIDFilePath はデーモンの PID ファイルパスを返す。
func PIDFilePath(configDir string) string {
	return filepath.Join(configDir, "moleport.pid")
}

// Daemon はデーモンプロセスの全コンポーネントを保持し、ライフサイクルを管理する。
type Daemon struct {
	configDir string
	version   string
	startedAt time.Time

	cfgMgr         core.ConfigManager
	sshMgr         core.SSHManager
	fwdMgr         core.ForwardManager
	versionChecker *update.VersionChecker

	broker  *ipc.EventBroker
	handler *ipchandler.Handler
	server  *ipc.IPCServer
	pidFile *PIDFile

	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	wg      sync.WaitGroup
	stopped bool
	purge   bool

	warnings []string
}

// New は新しい Daemon を生成する。
func New(configDir string, version string) (*Daemon, error) {
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	store := yamlstore.NewYAMLStore()
	cfgMgr := core.NewConfigManager(store, configDir)
	cfg, err := cfgMgr.LoadConfig()
	if err != nil {
		c := core.DefaultConfig()
		cfg = &c
	}

	// SSH config パスの ~ を展開
	sshConfigPath := cfg.SSHConfigPath
	if expanded, err := infra.ExpandTilde(sshConfigPath); err == nil {
		sshConfigPath = expanded
	}

	parser := sshconfig.NewSSHConfigParser()
	sshMgr := ssh.NewSSHManager(
		parser,
		func() core.SSHConnection { return infra.NewSSHConnection() },
		sshConfigPath,
		cfg.Reconnect,
		cfg.Hosts,
	)
	fwdMgr := forward.NewForwardManager(sshMgr)

	// 保存済みのフォワードルールを読み込む
	var warnings []string
	for _, rule := range cfg.Forwards {
		if _, err := fwdMgr.AddRule(rule); err != nil {
			slog.Warn("failed to load forward rule", "rule", rule.Name, "error", err)
			warnings = append(warnings, fmt.Sprintf("failed to load forward rule %q: %v", rule.Name, err))
		}
	}

	pidFile := NewPIDFile(PIDFilePath(configDir))
	versionChecker := update.New(version, cfg.UpdateCheck.Enabled, cfg.UpdateCheck.Interval.Duration)

	// Daemon を先に生成し、IPC コンポーネントに渡す
	d := &Daemon{
		configDir:      configDir,
		version:        version,
		cfgMgr:         cfgMgr,
		sshMgr:         sshMgr,
		fwdMgr:         fwdMgr,
		versionChecker: versionChecker,
		pidFile:        pidFile,
		warnings:       warnings,
	}

	// EventBroker: server.SendNotification をクロージャで渡す
	// server は New() 完了前に必ず設定されるため、Start() 後の呼び出しは安全
	broker := ipc.NewEventBroker(func(clientID string, notification protocol.Notification) error {
		return d.server.SendNotification(clientID, notification)
	})

	handler := ipchandler.NewHandler(sshMgr, fwdMgr, cfgMgr, broker, d, versionChecker)
	server := ipc.NewIPCServer(SocketPath(configDir), handler.Handle)

	// クライアント切断時にブローカーから購読を削除する
	server.OnClientDisconnected = func(clientID string) {
		broker.RemoveClient(clientID)
	}

	// Handler に通知送信用のサーバー参照を設定
	handler.SetSender(server)

	d.broker = broker
	d.handler = handler
	d.server = server

	return d, nil
}

// Start はデーモンを起動する。
func (d *Daemon) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.pidFile.Acquire(); err != nil {
		return fmt.Errorf("acquire pid file: %w", err)
	}

	d.ctx, d.cancel = context.WithCancel(ctx)
	d.startedAt = time.Now()
	d.stopped = false

	if err := d.server.Start(d.ctx); err != nil {
		d.pidFile.Release()
		return fmt.Errorf("start ipc server: %w", err)
	}

	// SSH ホストを読み込む（エラーは警告のみ）
	if _, err := d.sshMgr.LoadHosts(); err != nil {
		slog.Warn("failed to load SSH hosts", "error", err)
		d.warnings = append(d.warnings, fmt.Sprintf("failed to load SSH hosts: %v", err))
	}

	const versionCheckInterval = 10 * time.Second
	d.versionChecker.Start(d.ctx, versionCheckInterval)

	d.startEventRouting()
	d.restoreState()
	d.autoStartForwards()

	slog.Info("daemon started", "pid", os.Getpid(), "config_dir", d.configDir)
	return nil
}

// Stop はデーモンを停止する。べき等で複数回呼んでも安全。
func (d *Daemon) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return nil
	}
	d.stopped = true

	slog.Info("daemon stopping")

	d.versionChecker.Stop()

	// コンテキストを最初にキャンセルして全コンポーネントに停止を通知
	if d.cancel != nil {
		d.cancel()
	}

	if d.purge {
		if err := d.cfgMgr.DeleteState(); err != nil {
			slog.Warn("failed to delete state", "error", err)
		}
	} else {
		d.saveState()
	}

	if err := d.fwdMgr.StopAllForwards(); err != nil {
		slog.Warn("failed to stop all forwards", "error", err)
	}
	d.fwdMgr.Close()
	d.sshMgr.Close()

	// イベントルーティングゴルーチンの終了を待つ
	d.wg.Wait()

	if err := d.server.Stop(); err != nil {
		slog.Warn("failed to stop ipc server", "error", err)
	}

	if err := d.pidFile.Release(); err != nil {
		slog.Warn("failed to release pid file", "error", err)
	}

	slog.Info("daemon stopped")
	return nil
}

// Wait はシグナル (SIGTERM/SIGINT) を待ち、受信したら Stop() を呼ぶ。
func (d *Daemon) Wait() error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigCh:
		slog.Info("received signal", "signal", sig)
	case <-d.ctx.Done():
		slog.Info("context cancelled")
	}

	signal.Stop(sigCh)
	return d.Stop()
}

// Shutdown はデーモンのコンテキストをキャンセルし、Wait() 経由で graceful shutdown を開始する。
// purge が true の場合、停止時に状態ファイルを削除する。
func (d *Daemon) Shutdown(purge bool) error {
	d.mu.Lock()
	d.purge = purge
	cancel := d.cancel
	d.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return nil
}
