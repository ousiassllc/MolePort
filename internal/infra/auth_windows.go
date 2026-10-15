//go:build windows

package infra

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/Microsoft/go-winio"
)

const (
	// openSSHAgentPipe は Windows 標準の OpenSSH agent の named pipe。
	openSSHAgentPipe = `\\.\pipe\openssh-ssh-agent`

	// pipePathPrefix は named pipe パスの接頭辞。
	pipePathPrefix = `\\.\pipe\`
)

// dialSSHAgent は SSH エージェントに接続する。
// SSH_AUTH_SOCK が設定されていればそれを優先する（named pipe パスなら named pipe、それ以外は Unix ソケット）。
// 未設定の場合は OpenSSH agent の既定 named pipe に接続する。
func dialSSHAgent() (net.Conn, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		sock = openSSHAgentPipe
	}

	var (
		conn net.Conn
		err  error
	)
	if strings.HasPrefix(sock, pipePathPrefix) {
		conn, err = winio.DialPipe(sock, nil)
	} else {
		conn, err = net.Dial("unix", sock) //nolint:gosec // SSH_AUTH_SOCK はユーザー環境の信頼済み設定値
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH agent: %w", err)
	}
	return conn, nil
}
