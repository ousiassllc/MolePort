package infra

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"slices"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/ousiassllc/moleport/internal/core"
)

// ExpandTilde は ~ をホームディレクトリに展開する。
// "~/" または "~" のみ展開し、"~otheruser" パターンはそのまま返す。
func ExpandTilde(path string) (string, error) {
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
// 接続方法は OS ごとに異なる（auth_unix.go / auth_windows.go）。
// 呼び出し元は返された net.Conn を適切にクローズする責任を持つ。
func trySSHAgent() (ssh.AuthMethod, net.Conn, error) {
	conn, err := dialSSHAgent()
	if err != nil {
		return nil, nil, err
	}
	agentClient := agent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers), conn, nil
}

// loadKeySigner は秘密鍵ファイルから署名器を読み込む。
// 鍵がパスフレーズで暗号化されている場合、コールバックを使ってパスフレーズを取得する。
func loadKeySigner(path string, cb core.CredentialCallback, host core.SSHHost) (ssh.Signer, error) {
	keyData, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read key file %s: %w", path, err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		var passErr *ssh.PassphraseMissingError
		if errors.As(err, &passErr) && cb != nil {
			resp, cbErr := cb(core.CredentialRequest{
				Type:   core.CredentialPassphrase,
				Host:   host.Name,
				Prompt: "Enter passphrase for key '" + path + "':",
			})
			if cbErr != nil {
				return nil, fmt.Errorf("credential callback failed for %s: %w", path, cbErr)
			}
			if resp.Cancelled {
				return nil, fmt.Errorf("passphrase input cancelled for %s", path)
			}
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(resp.Value))
			if err != nil {
				return nil, fmt.Errorf("failed to parse key file %s with passphrase: %w", path, err)
			}
			return signer, nil
		}
		return nil, fmt.Errorf("failed to parse key file %s: %w", path, err)
	}
	return signer, nil
}

// SSH 認証方式名（RFC 4252）。サーバーが許可する方式との照合に使う。
const (
	authMethodPublicKey           = "publickey"
	authMethodPassword            = "password"
	authMethodKeyboardInteractive = "keyboard-interactive"
)

// authStep は認証の 1 段階。buildAuthMethods が試行順に並べて返す。
type authStep struct {
	method string         // RFC 4252 の認証方式名
	auth   ssh.AuthMethod // 実際の認証メソッド
}

// keyFileAuthStep は鍵ファイル 1 本分の認証段階を返す。
// 鍵の読み込み（パスフレーズの要求を含む）は、この段階が実際に試行されるまで行わない。
// 読み込みに失敗した鍵は署名器なしとして扱い、次の段階へ進める。
func keyFileAuthStep(path string, cb core.CredentialCallback, host core.SSHHost, logFailure bool) authStep {
	return authStep{authMethodPublicKey, ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
		signer, err := loadKeySigner(path, cb, host)
		if err != nil {
			if logFailure {
				slog.Debug("failed to load key file", "path", path, "error", err)
			}
			return nil, nil
		}
		return []ssh.Signer{signer}, nil
	})}
}

// sequentialAuthCallback は steps を先頭から順に 1 段階ずつ試行する ssh.ClientAuthCallback を返す。
// crypto/ssh は同じ方式（publickey 等）を 1 回しか試行しないため、ClientConfig.Auth に並べるだけでは
// エージェントの後の鍵ファイルが試行されない。コールバックで次の段階を選ぶことで、
// 前の段階が失敗した場合にだけ後続の段階（鍵の読み込み・クレデンシャル要求）が実行される。
// サーバーが許可していない方式の段階は選ばない。
func sequentialAuthCallback(steps []authStep) ssh.ClientAuthCallback {
	used := make([]bool, len(steps))
	return func(ctx *ssh.ClientAuthContext) (ssh.AuthMethod, error) {
		for i, step := range steps {
			if used[i] || !slices.Contains(ctx.AllowedMethods, step.method) {
				continue
			}
			used[i] = true
			return step.auth, nil
		}
		return nil, nil
	}
}

// buildAuthMethods はホスト情報をもとに認証の段階を試行順に構築する。
// 順序: SSH エージェント → ホスト固有の IdentityFiles → 既定の鍵パス →
// パスワード → keyboard-interactive（後ろ 2 つは cb が nil でない場合のみ）。
// 鍵ファイルの読み込みとパスフレーズの要求は、その段階が試行されるまで遅延する
// （エージェントや前の鍵で認証できれば、後続の鍵のパスフレーズは要求されない）。
// 返される io.Closer は SSH エージェント接続を閉じるために使用する。
// エージェントに接続しなかった場合は nil が返される。
func buildAuthMethods(host core.SSHHost, cb core.CredentialCallback) ([]authStep, io.Closer) {
	var steps []authStep
	var agentCloser io.Closer

	// SSH エージェントを試行
	if agentAuth, conn, err := trySSHAgent(); err == nil {
		steps = append(steps, authStep{authMethodPublicKey, agentAuth})
		agentCloser = conn
	}

	// ホスト固有の IdentityFiles
	for _, idFile := range host.IdentityFiles {
		if fileExists(idFile) {
			steps = append(steps, keyFileAuthStep(idFile, cb, host, true))
		}
	}

	// デフォルト鍵パス
	hostKeySet := make(map[string]bool, len(host.IdentityFiles))
	for _, f := range host.IdentityFiles {
		hostKeySet[f] = true
	}
	for _, keyPath := range defaultKeyPaths() {
		if hostKeySet[keyPath] || !fileExists(keyPath) {
			continue // 重複・存在しない鍵は除外
		}
		steps = append(steps, keyFileAuthStep(keyPath, cb, host, false))
	}

	if cb == nil {
		return steps, agentCloser
	}

	// パスワード認証
	steps = append(steps, authStep{authMethodPassword, ssh.PasswordCallback(func() (string, error) {
		resp, err := cb(core.CredentialRequest{
			Type:   core.CredentialPassword,
			Host:   host.Name,
			Prompt: "Password:",
		})
		if err != nil {
			return "", err
		}
		if resp.Cancelled {
			return "", fmt.Errorf("password input cancelled")
		}
		return resp.Value, nil
	})})

	// keyboard-interactive 認証
	steps = append(steps, authStep{authMethodKeyboardInteractive, ssh.KeyboardInteractive(
		func(name, instruction string, questions []string, echos []bool) ([]string, error) {
			prompts := make([]core.PromptInfo, len(questions))
			for i, q := range questions {
				prompts[i] = core.PromptInfo{Prompt: q, Echo: echos[i]}
			}
			resp, err := cb(core.CredentialRequest{
				Type:    core.CredentialKeyboardInteractive,
				Host:    host.Name,
				Prompts: prompts,
			})
			if err != nil {
				return nil, err
			}
			if resp.Cancelled {
				return nil, fmt.Errorf("keyboard-interactive input cancelled")
			}
			return resp.Answers, nil
		},
	)})

	return steps, agentCloser
}

// fileExists は path が存在するかを返す（読み込みは行わない）。
func fileExists(path string) bool {
	_, err := os.Stat(filepath.Clean(path))
	return err == nil
}
