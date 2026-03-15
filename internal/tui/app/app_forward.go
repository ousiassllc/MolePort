package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/i18n"
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
			Name:           msg.Name,
			Host:           msg.Host,
			Type:           msg.Type.String(),
			LocalPort:      msg.LocalPort,
			RemoteHost:     msg.RemoteHost,
			RemotePort:     msg.RemotePort,
			RemoteBindAddr: msg.RemoteBindAddr,
			AutoConnect:    msg.AutoConnect,
		}
		var result protocol.ForwardAddResult
		if err := m.client.Call(ctx, "forward.add", params, &result); err != nil {
			return tui.LogOutputMsg{Text: i18n.T("tui.log.forward_add_error", map[string]any{"Error": err}), Level: tui.LogError}
		}

		// AutoConnect が設定されている場合はフォワードも開始
		if msg.AutoConnect {
			if errMsg := m.startAndRollback(result); errMsg != nil {
				return *errMsg
			}
			return tui.LogOutputMsg{Text: i18n.T("tui.log.forward_added_started", map[string]any{"Name": result.Name}), Level: tui.LogSuccess}
		}

		return tui.LogOutputMsg{Text: i18n.T("tui.log.forward_added", map[string]any{"Name": result.Name}), Level: tui.LogSuccess}
	}
}

// startAndRollback はフォワードの開始を試み、失敗時にルールを削除してロールバックする。
// 成功時は nil を返す。
func (m *MainModel) startAndRollback(result protocol.ForwardAddResult) *tui.LogOutputMsg {
	startCtx, startCancel := context.WithTimeout(context.Background(), ipcCredentialTimeout)
	defer startCancel()
	startParams := protocol.ForwardStartParams(result)
	var startResult protocol.ForwardStartResult
	if err := m.client.Call(startCtx, "forward.start", startParams, &startResult); err != nil {
		delCtx, delCancel := context.WithTimeout(context.Background(), ipcWriteTimeout)
		defer delCancel()
		delParams := protocol.ForwardDeleteParams(result)
		var delResult protocol.ForwardDeleteResult
		if delErr := m.client.Call(delCtx, "forward.delete", delParams, &delResult); delErr != nil {
			return &tui.LogOutputMsg{Text: i18n.T("tui.log.forward_start_rollback_error", map[string]any{"Name": result.Name, "Error": err, "DeleteError": delErr}), Level: tui.LogError}
		}
		return &tui.LogOutputMsg{Text: i18n.T("tui.log.forward_start_error", map[string]any{"Name": result.Name, "Error": err}), Level: tui.LogError}
	}
	return nil
}

func (m *MainModel) deleteForwardRule(ruleName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcWriteTimeout)
		defer cancel()
		params := protocol.ForwardDeleteParams{Name: ruleName}
		var result protocol.ForwardDeleteResult
		if err := m.client.Call(ctx, "forward.delete", params, &result); err != nil {
			return tui.LogOutputMsg{Text: i18n.T("tui.log.forward_delete_error", map[string]any{"Name": ruleName, "Error": err}), Level: tui.LogError}
		}
		return tui.LogOutputMsg{Text: i18n.T("tui.log.forward_deleted", map[string]any{"Name": ruleName}), Level: tui.LogSuccess}
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
		ctx, cancel := context.WithTimeout(context.Background(), ipcCredentialTimeout)
		defer cancel()
		params := protocol.ForwardStartParams{Name: ruleName}
		var result protocol.ForwardStartResult
		if err := m.client.Call(ctx, "forward.start", params, &result); err != nil {
			return tui.LogOutputMsg{Text: i18n.T("tui.log.forward_start_error", map[string]any{"Name": ruleName, "Error": err}), Level: tui.LogError}
		}
		return tui.LogOutputMsg{Text: i18n.T("tui.log.forward_started", map[string]any{"Name": ruleName}), Level: tui.LogSuccess}
	}
}

func (m *MainModel) stopForward(ruleName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ipcWriteTimeout)
		defer cancel()
		params := protocol.ForwardStopParams{Name: ruleName}
		var result protocol.ForwardStopResult
		if err := m.client.Call(ctx, "forward.stop", params, &result); err != nil {
			return tui.LogOutputMsg{Text: i18n.T("tui.log.forward_stop_error", map[string]any{"Name": ruleName, "Error": err}), Level: tui.LogError}
		}
		return tui.LogOutputMsg{Text: i18n.T("tui.log.forward_stopped", map[string]any{"Name": ruleName}), Level: tui.LogSuccess}
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
		prompt = i18n.T("tui.log.credential_passphrase_prompt", map[string]any{"Host": msg.Request.Host})
	case "keyboard-interactive":
		if len(msg.Request.Prompts) > 0 {
			prompt = msg.Request.Prompts[0].Prompt
		} else {
			prompt = i18n.T("tui.log.credential_code_prompt", map[string]any{"Host": msg.Request.Host})
		}
	default:
		prompt = i18n.T("tui.log.credential_password_prompt", map[string]any{"Host": msg.Request.Host})
	}

	cmd := m.dashboard.ShowPasswordInput(prompt)
	m.dashboard.AppendLog(i18n.T("tui.log.credential_required", map[string]any{"Host": msg.Request.Host, "Type": msg.Request.Type}), tui.LogInfo)
	return m, cmd
}

func (m MainModel) handleCredentialSubmit(msg tui.CredentialSubmitMsg) (tea.Model, tea.Cmd) {
	if m.credResponseCh == nil {
		return m, nil
	}

	if msg.Cancelled {
		m.credResponseCh <- nil
		m.dashboard.AppendLog(i18n.T("tui.log.credential_cancelled"), tui.LogInfo)
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
