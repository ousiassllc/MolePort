//go:build !windows

package infra

import (
	"fmt"
	"net"
	"os"
)

// dialSSHAgent は SSH_AUTH_SOCK の Unix ソケットで SSH エージェントに接続する。
func dialSSHAgent() (net.Conn, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}
	conn, err := net.Dial("unix", sock) //nolint:gosec // SSH_AUTH_SOCK はユーザー環境の信頼済み設定値
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH agent: %w", err)
	}
	return conn, nil
}
