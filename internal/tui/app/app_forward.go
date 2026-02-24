package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/client"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
	"github.com/ousiassllc/moleport/internal/tui"
)

// --- フォワード操作 ---

func (m *MainModel) handleForwardAdd(msg tui.ForwardAddRequestMsg) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcWriteTimeout)
		defer cancel()
		params := protocol.ForwardAddParams{
			Name:        msg.Name,
			Host:        msg.Host,
			Type:        msg.Type.String(),
			LocalPort:   msg.LocalPort,
			RemoteHost:  msg.RemoteHost,
			RemotePort:  msg.RemotePort,
			AutoConnect: msg.AutoConnect,
		}
		var result protocol.ForwardAddResult
		if err := m.client.Call(ctx, "forward.add", params, &result); err != nil {
			return tui.LogOutputMsg{Text: fmt.Sprintf("ルール追加エラー: %s", err)}
		}

		// AutoConnect が設定されている場合はフォワードも開始
		if msg.AutoConnect {
			startParams := protocol.ForwardStartParams{Name: result.Name}
			var startResult protocol.ForwardStartResult
			if err := m.client.Call(ctx, "forward.start", startParams, &startResult); err != nil {
				// 開始に失敗したルールを削除（ロールバック）
				delCtx, delCancel := context.WithTimeout(context.Background(), ipcWriteTimeout)
				defer delCancel()
				delParams := protocol.ForwardDeleteParams(result)
				var delResult protocol.ForwardDeleteResult
				if delErr := m.client.Call(delCtx, "forward.delete", delParams, &delResult); delErr != nil {
					return tui.LogOutputMsg{Text: fmt.Sprintf("ルール '%s' の開始に失敗: %s（ルール削除にも失敗: %s）", result.Name, err, delErr)}
				}
				return tui.LogOutputMsg{Text: fmt.Sprintf("ルール '%s' の開始に失敗: %s", result.Name, err)}
			}
			return tui.LogOutputMsg{Text: fmt.Sprintf("ルール '%s' を追加し、開始しました", result.Name)}
		}

		return tui.LogOutputMsg{Text: fmt.Sprintf("ルール '%s' を追加しました", result.Name)}
	}
}

func (m *MainModel) deleteForwardRule(ruleName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcWriteTimeout)
		defer cancel()
		params := protocol.ForwardDeleteParams{Name: ruleName}
		var result protocol.ForwardDeleteResult
		if err := m.client.Call(ctx, "forward.delete", params, &result); err != nil {
			return tui.LogOutputMsg{Text: fmt.Sprintf("ルール削除エラー: %s", err)}
		}
		return tui.LogOutputMsg{Text: fmt.Sprintf("ルール '%s' を削除しました", ruleName)}
	}
}

func (m *MainModel) toggleForward(ruleName string) tea.Cmd {
	// ローカルのセッション情報から状態を判定する
	for _, s := range m.sessions {
		if s.Rule.Name == ruleName {
			if s.Status == core.Active {
				return m.stopForward(ruleName)
			}
			return m.startForward(ruleName)
		}
	}
	return m.startForward(ruleName)
}

func (m *MainModel) startForward(ruleName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcWriteTimeout)
		defer cancel()
		params := protocol.ForwardStartParams{Name: ruleName}
		var result protocol.ForwardStartResult
		if err := m.client.Call(ctx, "forward.start", params, &result); err != nil {
			return tui.LogOutputMsg{Text: fmt.Sprintf("フォワード開始エラー (%s): %s", ruleName, err)}
		}
		return tui.LogOutputMsg{Text: fmt.Sprintf("フォワード '%s' を開始しました", ruleName)}
	}
}

func (m *MainModel) stopForward(ruleName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcWriteTimeout)
		defer cancel()
		params := protocol.ForwardStopParams{Name: ruleName}
		var result protocol.ForwardStopResult
		if err := m.client.Call(ctx, "forward.stop", params, &result); err != nil {
			return tui.LogOutputMsg{Text: fmt.Sprintf("フォワード停止エラー: %s", err)}
		}
		return tui.LogOutputMsg{Text: fmt.Sprintf("フォワード '%s' を停止しました", ruleName)}
	}
}

// --- クレデンシャル入力 ---

// NewTUICredentialHandler は Bubble Tea プログラムにクレデンシャル要求を送信するハンドラーを返す。
// tui_cmd.go から tea.Program 生成後に呼び出す。
func NewTUICredentialHandler(p *tea.Program) client.CredentialHandler {
	return func(req protocol.CredentialRequestNotification) (*protocol.CredentialResponseParams, error) {
		ch := make(chan *protocol.CredentialResponseParams, 1)
		p.Send(tui.CredentialRequestMsg{
			Request:    req,
			ResponseCh: ch,
		})
		resp := <-ch
		return resp, nil
	}
}

func (m MainModel) handleCredentialRequest(msg tui.CredentialRequestMsg) (tea.Model, tea.Cmd) {
	m.credRequest = &msg.Request
	m.credResponseCh = msg.ResponseCh

	var prompt string
	switch msg.Request.Type {
	case "passphrase":
		prompt = fmt.Sprintf("%s の鍵パスフレーズを入力:", msg.Request.Host)
	case "keyboard-interactive":
		if len(msg.Request.Prompts) > 0 {
			prompt = msg.Request.Prompts[0].Prompt
		} else {
			prompt = fmt.Sprintf("%s の認証コードを入力:", msg.Request.Host)
		}
	default:
		prompt = fmt.Sprintf("%s のパスワードを入力:", msg.Request.Host)
	}

	cmd := m.dashboard.ShowPasswordInput(prompt)
	m.dashboard.AppendLog(fmt.Sprintf("認証が必要です: %s (%s)", msg.Request.Host, msg.Request.Type))
	return m, cmd
}

func (m MainModel) handleCredentialSubmit(msg tui.CredentialSubmitMsg) (tea.Model, tea.Cmd) {
	if m.credResponseCh == nil {
		return m, nil
	}

	if msg.Cancelled {
		m.credResponseCh <- nil
		m.dashboard.AppendLog("認証がキャンセルされました")
	} else {
		resp := &protocol.CredentialResponseParams{
			Value: msg.Value,
		}
		if m.credRequest != nil {
			resp.RequestID = m.credRequest.RequestID
			// keyboard-interactive の場合は Answers に入れる
			if m.credRequest.Type == "keyboard-interactive" {
				resp.Answers = []string{msg.Value}
				resp.Value = ""
			}
		}
		m.credResponseCh <- resp
	}

	m.credRequest = nil
	m.credResponseCh = nil
	return m, nil
}
