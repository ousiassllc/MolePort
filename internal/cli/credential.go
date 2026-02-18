package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/ousiassllc/moleport/internal/ipc"
)

// newCLICredentialHandler はターミナルからクレデンシャルを読み取る CredentialHandler を返す。
func newCLICredentialHandler() ipc.CredentialHandler {
	return func(req ipc.CredentialRequestNotification) (*ipc.CredentialResponseParams, error) {
		switch req.Type {
		case "password", "passphrase":
			return handlePasswordPrompt(req)
		case "keyboard-interactive":
			return handleKeyboardInteractive(req)
		default:
			return nil, fmt.Errorf("unknown credential type: %s", req.Type)
		}
	}
}

// handlePasswordPrompt はパスワード/パスフレーズのサイレント入力を行う。
func handlePasswordPrompt(req ipc.CredentialRequestNotification) (*ipc.CredentialResponseParams, error) {
	prompt := req.Prompt
	if prompt == "" {
		if req.Type == "passphrase" {
			prompt = fmt.Sprintf("%s の鍵パスフレーズ: ", req.Host)
		} else {
			prompt = fmt.Sprintf("%s のパスワード: ", req.Host)
		}
	}

	fmt.Fprint(os.Stderr, prompt)
	password, err := term.ReadPassword(int(os.Stdin.Fd())) //nolint:gosec // stdin fd is always 0
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, err
	}

	return &ipc.CredentialResponseParams{
		RequestID: req.RequestID,
		Value:     string(password),
	}, nil
}

// handleKeyboardInteractive は keyboard-interactive 認証のプロンプトを処理する。
func handleKeyboardInteractive(req ipc.CredentialRequestNotification) (*ipc.CredentialResponseParams, error) {
	if len(req.Prompts) == 0 {
		return &ipc.CredentialResponseParams{
			RequestID: req.RequestID,
			Answers:   []string{},
		}, nil
	}

	answers := make([]string, len(req.Prompts))
	reader := bufio.NewReader(os.Stdin)

	for i, p := range req.Prompts {
		fmt.Fprint(os.Stderr, p.Prompt)

		if p.Echo {
			line, err := reader.ReadString('\n')
			if err != nil {
				return nil, err
			}
			answers[i] = strings.TrimRight(line, "\r\n")
		} else {
			password, err := term.ReadPassword(int(os.Stdin.Fd())) //nolint:gosec // stdin fd is always 0
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return nil, err
			}
			answers[i] = string(password)
		}
	}

	return &ipc.CredentialResponseParams{
		RequestID: req.RequestID,
		Answers:   answers,
	}, nil
}
