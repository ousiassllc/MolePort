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
	"github.com/ousiassllc/moleport/internal/daemon/pidfile"
	"github.com/ousiassllc/moleport/internal/infra"
	"github.com/ousiassllc/moleport/internal/infra/permcheck"
	"github.com/ousiassllc/moleport/internal/infra/sshconfig"
	"github.com/ousiassllc/moleport/internal/infra/yamlstore"
	"github.com/ousiassllc/moleport/internal/ipc"
	ipchandler "github.com/ousiassllc/moleport/internal/ipc/handler"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

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
	pidFile *pidfile.PIDFile

	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	wg      sync.WaitGroup
	stopped bool
	purge   bool

	// shutdownTimeout は切断処理の上限。0 以下なら defaultShutdownTimeout を使う（テスト用に上書き可）。
	shutdownTimeout time.Duration

	warnings []string
}

// New は新しい Daemon を生成する。
func New(configDir string, version string) (*Daemon, error) {
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	store := yamlstore.NewYAMLStore()
	cfgMgr := core.NewConfigManager(store, configDir)
	var warnings []string
	cfg, err := cfgMgr.LoadConfig()
	if err != nil {
		// 読み込めなかった config.yaml は最初の書き込み前に config.yaml.bak へ退避される（yamlstore）。
		slog.Error("failed to load config; using defaults", "path", filepath.Join(configDir, "config.yaml"), "error", err)
		warnings = append(warnings, fmt.Sprintf("failed to load config.yaml (using defaults; the original file is kept as config.yaml.bak when settings are saved): %v", err))
		c := core.DefaultConfig()
		cfg = &c
	}

	// SSH config パスの ~ を展開
	sshConfigPath := cfg.SSHConfigPath
	if expanded, err := infra.ExpandTilde(sshConfigPath); err == nil {
		sshConfigPath = expanded
	}

	parser := sshconfig.NewSSHConfigParser()

	ctx, cancel := context.WithCancel(context.Background())
	sshMgr := ssh.NewSSHManager(
		ctx,
		parser,
		func() core.SSHConnection { return infra.NewSSHConnection() },
		sshConfigPath,
		cfg.Reconnect,
		cfg.Hosts,
	)
	fwdMgr := forward.NewForwardManager(ctx, sshMgr)

	// 保存済みのフォワードルールを読み込む
	for _, rule := range cfg.Forwards {
		if _, err := fwdMgr.AddRule(rule); err != nil {
			slog.Warn("failed to load forward rule", "rule", rule.Name, "error", err)
			warnings = append(warnings, fmt.Sprintf("failed to load forward rule %q: %v", rule.Name, err))
		}
	}

	// config.yaml を直接編集した無効な間隔（0 以下・1h 未満）は補正して警告する（NFR-31）。
	checkInterval, adjusted := update.NormalizeInterval(cfg.UpdateCheck.Interval.Duration)
	if adjusted {
		slog.Warn("update_check.interval is invalid; adjusted", "configured", cfg.UpdateCheck.Interval.String(), "using", checkInterval.String())
		warnings = append(warnings, fmt.Sprintf("update_check.interval %s is invalid (minimum %s); using %s", cfg.UpdateCheck.Interval.Duration, update.MinInterval, checkInterval))
	}
	// 設定ディレクトリ・設定ファイル・SSH config のパーミッションが緩い場合は警告する（NFR-09 / NFR-10）
	warnings = append(warnings, permcheck.Warnings(configDir, sshConfigPath)...)

	pidFile := pidfile.NewPIDFile(PIDFilePath(configDir))
	versionChecker := update.New(version, cfg.UpdateCheck.Enabled, checkInterval)

	// Daemon を先に生成し、IPC コンポーネントに渡す
	d := &Daemon{
		configDir:      configDir,
		version:        version,
		cfgMgr:         cfgMgr,
		sshMgr:         sshMgr,
		fwdMgr:         fwdMgr,
		versionChecker: versionChecker,
		pidFile:        pidFile,
		ctx:            ctx,
		cancel:         cancel,
		warnings:       warnings,
	}

	// EventBroker: server.SendNotification をクロージャで渡す
	// server は New() 完了前に必ず設定されるため、Start() 後の呼び出しは安全
	broker := ipc.NewEventBroker(func(clientID string, notification protocol.Notification) error {
		return d.server.SendNotification(clientID, notification)
	})

	// 送信キューが溢れた（読み取りが滞っている）クライアントは切断する
	broker.SetOnOverflow(func(clientID string) {
		d.server.DisconnectClient(clientID)
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

	// 呼び出し元のコンテキストがキャンセルされたら、デーモンのコンテキストもキャンセルする
	go func() {
		select {
		case <-ctx.Done():
			d.cancel()
		case <-d.ctx.Done():
		}
	}()
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
	d.startBackgroundRestore()

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

	d.closeComponents()

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
