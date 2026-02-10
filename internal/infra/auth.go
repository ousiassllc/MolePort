package infra

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/ousiassllc/moleport/internal/core"
)

// expandTilde は ~ をホームディレクトリに展開する。
// "~/" または "~" のみ展開し、"~otheruser" パターンはそのまま返す。
func expandTilde(path string) (string, error) {
	if len(path) == 0 {
		return path, nil
	}
	if path == "~" {
		home := homeDir()
		if home == "" {
			return "", fmt.Errorf("failed to get home directory")
		}
		return home, nil
	}
	if len(path) >= 2 && path[0] == '~' && path[1] == '/' {
		home := homeDir()
		if home == "" {
			return "", fmt.Errorf("failed to get home directory")
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// defaultKeyPaths は一般的な SSH 秘密鍵のパスを返す。
func defaultKeyPaths() []string {
	home := homeDir()
	if home == "" {
		return nil
	}
	sshDir := filepath.Join(home, ".ssh")
	return []string{
		filepath.Join(sshDir, "id_rsa"),
		filepath.Join(sshDir, "id_ed25519"),
		filepath.Join(sshDir, "id_ecdsa"),
		filepath.Join(sshDir, "id_dsa"),
	}
}

// trySSHAgent は SSH エージェントからの認証メソッドと接続を取得する。
// 呼び出し元は返された net.Conn を適切にクローズする責任を持つ。
func trySSHAgent() (ssh.AuthMethod, net.Conn, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to SSH agent: %w", err)
	}
	agentClient := agent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers), conn, nil
}

// tryKeyFile は秘密鍵ファイルから認証メソッドを取得する。
func tryKeyFile(path string) (ssh.AuthMethod, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file %s: %w", path, err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key file %s: %w", path, err)
	}
	return ssh.PublicKeys(signer), nil
}

// buildAuthMethods はホスト情報をもとに認証メソッドのリストを構築する。
// SSH エージェントと鍵ファイルを組み合わせる。
// 返される io.Closer は SSH エージェント接続を閉じるために使用する。
// エージェントに接続しなかった場合は nil が返される。
func buildAuthMethods(host core.SSHHost) ([]ssh.AuthMethod, io.Closer) {
	var methods []ssh.AuthMethod
	var agentCloser io.Closer

	// SSH エージェントを試行
	if agentAuth, conn, err := trySSHAgent(); err == nil {
		methods = append(methods, agentAuth)
		agentCloser = conn
	}

	// ホスト固有の IdentityFile
	if host.IdentityFile != "" {
		if keyAuth, err := tryKeyFile(host.IdentityFile); err == nil {
			methods = append(methods, keyAuth)
		} else {
			slog.Debug("failed to load identity file", "path", host.IdentityFile, "error", err)
		}
	}

	// デフォルト鍵パス
	for _, keyPath := range defaultKeyPaths() {
		if host.IdentityFile == keyPath {
			continue // 重複を避ける
		}
		if keyAuth, err := tryKeyFile(keyPath); err == nil {
			methods = append(methods, keyAuth)
		}
	}

	return methods, agentCloser
}
