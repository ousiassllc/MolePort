package ssh

import (
	"context"
	"sync"
	"time"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/ousiassllc/moleport/internal/core"
)

const (
	// defaultKeepAliveInterval は KeepAliveInterval が未設定時のフォールバック値。
	defaultKeepAliveInterval = 30 * time.Second
)

// keepAliveInterval は設定された KeepAlive 間隔を返す。未設定の場合はデフォルト値を返す。
func (m *sshManager) keepAliveInterval() time.Duration {
	if d := m.reconnectCfg.KeepAliveInterval.Duration; d > 0 {
		return d
	}
	return defaultKeepAliveInterval
}

// hostConnection は個々のホストへの接続状態を保持する。
type hostConnection struct {
	conn   core.SSHConnection
	client *cryptossh.Client
	ctx    context.Context
	cancel context.CancelFunc
	state  core.ConnectionState
}

type sshManager struct {
	mu           sync.RWMutex
	ctx          context.Context
	parser       core.SSHConfigParser
	connFactory  func() core.SSHConnection
	configPath   string
	reconnectCfg core.ReconnectConfig
	hostConfigs  map[string]core.HostConfig

	hosts            []core.SSHHost
	hostsMap         map[string]int
	conns            map[string]*hostConnection
	reconnectCancels map[string]context.CancelFunc // ホストごとの再接続キャンセル関数
	events           core.EventEmitter[core.SSHEvent]

	closed bool
}

// NewSSHManager は SSHManager の実装を返す。
func NewSSHManager(
	ctx context.Context,
	parser core.SSHConfigParser,
	connFactory func() core.SSHConnection,
	configPath string,
	reconnectCfg core.ReconnectConfig,
	hostConfigs map[string]core.HostConfig,
) core.SSHManager {
	if hostConfigs == nil {
		hostConfigs = make(map[string]core.HostConfig)
	}
	m := &sshManager{
		ctx:              ctx,
		parser:           parser,
		connFactory:      connFactory,
		configPath:       configPath,
		reconnectCfg:     reconnectCfg,
		hostConfigs:      hostConfigs,
		hostsMap:         make(map[string]int),
		conns:            make(map[string]*hostConnection),
		reconnectCancels: make(map[string]context.CancelFunc),
	}
	m.events = core.NewEventEmitter[core.SSHEvent](&m.mu)
	return m
}

// copyHosts はホスト一覧のコピーを返す。mu.Lock の中で呼ぶこと。
func (m *sshManager) copyHosts() []core.SSHHost {
	result := make([]core.SSHHost, len(m.hosts))
	copy(result, m.hosts)
	return result
}
