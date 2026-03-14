package ssh

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/core"
)

// LoadHosts は SSH config を解析してホスト一覧を構築する。
func (m *sshManager) LoadHosts() ([]core.SSHHost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	hosts, err := m.parser.Parse(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH config: %w", err)
	}

	m.hosts = hosts
	m.hostsMap = make(map[string]int, len(hosts))
	for i, h := range hosts {
		m.hostsMap[h.Name] = i
	}

	return m.copyHosts(), nil
}

// ReloadHosts は SSH config を再解析し、既存の接続状態を保持する。
func (m *sshManager) ReloadHosts() ([]core.SSHHost, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	hosts, err := m.parser.Parse(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH config: %w", err)
	}

	// 既存の接続状態を保持
	oldStates := make(map[string]core.ConnectionState)
	oldForwards := make(map[string]int)
	for _, h := range m.hosts {
		oldStates[h.Name] = h.State
		oldForwards[h.Name] = h.ActiveForwardCount
	}

	for i := range hosts {
		if state, ok := oldStates[hosts[i].Name]; ok {
			hosts[i].State = state
			hosts[i].ActiveForwardCount = oldForwards[hosts[i].Name]
		}
	}

	m.hosts = hosts
	m.hostsMap = make(map[string]int, len(hosts))
	for i, h := range hosts {
		m.hostsMap[h.Name] = i
	}

	return m.copyHosts(), nil
}

// GetHosts はキャッシュ済みホスト一覧のコピーを返す。ファイルの再解析は行わない。
// LoadHosts または ReloadHosts でキャッシュを構築してから呼び出すこと。
func (m *sshManager) GetHosts() []core.SSHHost {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.copyHosts()
}

// GetHost は名前でホストを取得する。
func (m *sshManager) GetHost(name string) (*core.SSHHost, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idx, ok := m.hostsMap[name]
	if !ok {
		return nil, &core.NotFoundError{Resource: "host", Name: name}
	}
	h := m.hosts[idx]
	return &h, nil
}
