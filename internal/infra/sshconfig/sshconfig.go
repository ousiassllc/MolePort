package sshconfig

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	ssh_config "github.com/kevinburke/ssh_config"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/infra"
)

type sshConfigParser struct{}

// NewSSHConfigParser は core.SSHConfigParser の実装を返す。
func NewSSHConfigParser() core.SSHConfigParser {
	return &sshConfigParser{}
}

func (p *sshConfigParser) Parse(configPath string) ([]core.SSHHost, error) {
	f, err := os.Open(configPath) //nolint:gosec // configPath は SSH config のパスでユーザー指定値
	if err != nil {
		return nil, fmt.Errorf("failed to open ssh config: %w", err)
	}
	defer f.Close() //nolint:errcheck // 読み取り専用のため Close エラーは無視

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ssh config: %w", err)
	}

	currentUser := currentUsername()

	var hosts []core.SSHHost
	seen := make(map[string]bool)

	for _, host := range cfg.Hosts {
		for _, pattern := range host.Patterns {
			alias := pattern.String()

			// ワイルドカードや否定パターンを除外
			if strings.ContainsAny(alias, "*?!") {
				continue
			}
			if seen[alias] {
				continue
			}
			seen[alias] = true

			sshHost := core.SSHHost{
				Name:                  alias,
				HostName:              getConfigValue(cfg, alias, "HostName", alias),
				Port:                  getConfigPort(cfg, alias),
				User:                  getConfigValue(cfg, alias, "User", currentUser),
				IdentityFiles:         expandIdentityFiles(cfg, alias),
				ProxyJump:             parseProxyJump(getConfigValue(cfg, alias, "ProxyJump", "")),
				ProxyCommand:          getConfigValue(cfg, alias, "ProxyCommand", ""),
				StrictHostKeyChecking: getConfigValue(cfg, alias, "StrictHostKeyChecking", ""),
				State:                 core.Disconnected,
				ActiveForwardCount:    0,
			}

			hosts = append(hosts, sshHost)
		}
	}

	return hosts, nil
}

func currentUsername() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return u.Username
}

func getConfigValue(cfg *ssh_config.Config, alias, key, defaultVal string) string {
	val, err := cfg.Get(alias, key)
	if err != nil || val == "" {
		return defaultVal
	}
	return val
}

func getConfigPort(cfg *ssh_config.Config, alias string) int {
	val, err := cfg.Get(alias, "Port")
	if err != nil || val == "" {
		return 22
	}
	port, err := strconv.Atoi(val)
	if err != nil {
		return 22
	}
	return port
}

func expandIdentityFile(path string) string {
	if path == "" {
		return ""
	}
	expanded, err := infra.ExpandTilde(path)
	if err != nil {
		return path
	}
	return expanded
}

func expandIdentityFiles(cfg *ssh_config.Config, alias string) []string {
	vals, err := cfg.GetAll(alias, "IdentityFile")
	if err != nil || len(vals) == 0 {
		return nil
	}
	result := make([]string, 0, len(vals))
	for _, v := range vals {
		if expanded := expandIdentityFile(v); expanded != "" {
			result = append(result, expanded)
		}
	}
	return result
}

func parseProxyJump(val string) []string {
	if val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
