package organisms

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/tui"
	"github.com/ousiassllc/moleport/internal/tui/molecules"
)

const maxOutputLines = 50

// promptStep はフロー内の1ステップを定義する。
type promptStep struct {
	key      string
	label    string
	validate func(string) error
}

// promptFlow は対話型コマンドのステップ一覧。
type promptFlow struct {
	name    string
	steps   []promptStep
	current int
	values  map[string]string
}

// CommandPanel はコマンド入力と対話型プロンプトを提供するパネル。
type CommandPanel struct {
	prompt    molecules.PromptInput
	output    []string
	flow      *promptFlow
	focused   bool
	width     int
	height    int
	hostNames []string
	rules     []core.ForwardRule
	sessions  []core.ForwardSession
}

// NewCommandPanel は新しい CommandPanel を生成する。
func NewCommandPanel() CommandPanel {
	return CommandPanel{
		prompt: molecules.NewPromptInput(),
	}
}

// SetFocused はフォーカス状態を設定する。
func (p *CommandPanel) SetFocused(focused bool) {
	p.focused = focused
	if focused {
		p.prompt.Focus()
	} else {
		p.prompt.Blur()
	}
}

// SetSize はパネルのサイズを設定する。
func (p *CommandPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// SetHostNames はホスト名リストを設定する（フロー用）。
func (p *CommandPanel) SetHostNames(names []string) {
	p.hostNames = names
}

// SetRules はルール一覧を設定する（フロー用）。
func (p *CommandPanel) SetRules(rules []core.ForwardRule) {
	p.rules = rules
}

// SetSessions はセッション一覧を設定する（フロー用）。
func (p *CommandPanel) SetSessions(sessions []core.ForwardSession) {
	p.sessions = sessions
}

// AppendOutput は出力バッファにテキストを追加する。
func (p *CommandPanel) AppendOutput(text string) {
	lines := strings.Split(text, "\n")
	p.output = append(p.output, lines...)
	if len(p.output) > maxOutputLines {
		p.output = p.output[len(p.output)-maxOutputLines:]
	}
}

// コマンドエイリアスの定義
var commandAliases = map[string]string{
	"a":    "add",
	"rm":   "delete",
	"del":  "delete",
	"conn": "connect",
	"disc": "disconnect",
	"dc":   "disconnect",
	"ls":   "list",
	"cfg":  "config",
	"rl":   "reload",
	"h":    "help",
	"q":    "quit",
	"st":   "status",
}

// resolveCommand はエイリアスを解決してコマンド名を返す。
func resolveCommand(input string) string {
	cmd := strings.TrimSpace(strings.ToLower(input))
	if resolved, ok := commandAliases[cmd]; ok {
		return resolved
	}
	return cmd
}

// Update はキー入力を処理する。
func (p CommandPanel) Update(msg tea.Msg) (CommandPanel, tea.Cmd) {
	if !p.focused {
		return p, nil
	}

	// PromptSubmitMsg の処理
	if submitMsg, ok := msg.(molecules.PromptSubmitMsg); ok {
		return p.handleSubmit(submitMsg.Value)
	}

	// フロー中のエスケープ処理
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyEsc {
		if p.flow != nil {
			p.flow = nil
			p.AppendOutput("キャンセルしました")
			return p, nil
		}
	}

	var cmd tea.Cmd
	p.prompt, cmd = p.prompt.Update(msg)
	return p, cmd
}

// handleSubmit は入力値を処理する。
func (p CommandPanel) handleSubmit(value string) (CommandPanel, tea.Cmd) {
	// フロー中の場合はフローに入力を渡す
	if p.flow != nil {
		return p.handleFlowInput(value)
	}

	// コマンド解析
	cmd := resolveCommand(value)
	p.AppendOutput("> " + value)

	switch cmd {
	case "add":
		return p.startAddFlow()
	case "delete":
		return p.startDeleteFlow()
	case "connect":
		return p.startConnectFlow()
	case "disconnect":
		return p.startDisconnectFlow()
	case "config":
		return p.startConfigFlow()
	case "help":
		return p.executeImmediate("help", nil)
	case "quit":
		return p.executeImmediate("quit", nil)
	case "reload":
		return p.executeImmediate("reload", nil)
	case "list":
		return p.executeImmediate("list", nil)
	case "status":
		return p.executeImmediate("status", nil)
	default:
		p.AppendOutput(fmt.Sprintf("不明なコマンド: %s (help で一覧表示)", cmd))
		return p, nil
	}
}

// executeImmediate は即座に実行されるコマンドの CommandExecuteMsg を発行する。
func (p CommandPanel) executeImmediate(command string, values map[string]string) (CommandPanel, tea.Cmd) {
	return p, func() tea.Msg {
		return tui.CommandExecuteMsg{Command: command, Values: values}
	}
}

// startAddFlow は add コマンドのフローを開始する。
func (p CommandPanel) startAddFlow() (CommandPanel, tea.Cmd) {
	if len(p.hostNames) == 0 {
		p.AppendOutput("利用可能なホストがありません")
		return p, nil
	}

	p.AppendOutput("--- フォワード追加 ---")
	p.AppendOutput("利用可能なホスト: " + strings.Join(p.hostNames, ", "))

	p.flow = &promptFlow{
		name: "add",
		steps: []promptStep{
			{key: "host", label: "ホスト名"},
			{key: "type", label: "種別 (local/remote/dynamic)", validate: validateForwardType},
			{key: "local_port", label: "ローカルポート", validate: validatePort},
			{key: "remote_host", label: "リモートホスト (default: localhost)"},
			{key: "remote_port", label: "リモートポート", validate: validatePort},
			{key: "name", label: "ルール名 (空欄で自動生成)"},
			{key: "auto_connect", label: "自動接続 (y/n, default: n)"},
		},
		values: make(map[string]string),
	}

	p.AppendOutput(p.flow.steps[0].label + ":")
	return p, nil
}

// startDeleteFlow は delete コマンドのフローを開始する。
func (p CommandPanel) startDeleteFlow() (CommandPanel, tea.Cmd) {
	if len(p.rules) == 0 {
		p.AppendOutput("削除可能なルールがありません")
		return p, nil
	}

	p.AppendOutput("--- ルール削除 ---")
	names := ruleNames(p.rules)
	p.AppendOutput("ルール一覧: " + strings.Join(names, ", "))

	p.flow = &promptFlow{
		name: "delete",
		steps: []promptStep{
			{key: "rule_name", label: "削除するルール名"},
		},
		values: make(map[string]string),
	}

	p.AppendOutput(p.flow.steps[0].label + ":")
	return p, nil
}

// startConnectFlow は connect コマンドのフローを開始する。
func (p CommandPanel) startConnectFlow() (CommandPanel, tea.Cmd) {
	stopped := filterSessionsByStatus(p.sessions, core.Stopped)
	if len(stopped) == 0 {
		p.AppendOutput("接続可能な（停止中の）ルールがありません")
		return p, nil
	}

	p.AppendOutput("--- フォワード接続 ---")
	names := sessionRuleNames(stopped)
	p.AppendOutput("停止中のルール: " + strings.Join(names, ", "))

	p.flow = &promptFlow{
		name: "connect",
		steps: []promptStep{
			{key: "rule_name", label: "接続するルール名"},
		},
		values: make(map[string]string),
	}

	p.AppendOutput(p.flow.steps[0].label + ":")
	return p, nil
}

// startDisconnectFlow は disconnect コマンドのフローを開始する。
func (p CommandPanel) startDisconnectFlow() (CommandPanel, tea.Cmd) {
	active := filterSessionsByStatus(p.sessions, core.Active)
	if len(active) == 0 {
		p.AppendOutput("切断可能な（アクティブの）ルールがありません")
		return p, nil
	}

	p.AppendOutput("--- フォワード切断 ---")
	names := sessionRuleNames(active)
	p.AppendOutput("アクティブなルール: " + strings.Join(names, ", "))

	p.flow = &promptFlow{
		name: "disconnect",
		steps: []promptStep{
			{key: "rule_name", label: "切断するルール名"},
		},
		values: make(map[string]string),
	}

	p.AppendOutput(p.flow.steps[0].label + ":")
	return p, nil
}

// startConfigFlow は config コマンドのフローを開始する。
func (p CommandPanel) startConfigFlow() (CommandPanel, tea.Cmd) {
	p.AppendOutput("--- 設定変更 ---")
	p.AppendOutput("カテゴリ: reconnect, session, log")

	p.flow = &promptFlow{
		name: "config",
		steps: []promptStep{
			{key: "category", label: "カテゴリ (reconnect/session/log)"},
			{key: "value", label: "新しい値"},
		},
		values: make(map[string]string),
	}

	p.AppendOutput(p.flow.steps[0].label + ":")
	return p, nil
}

// handleFlowInput はフロー内のステップに対して入力を処理する。
func (p CommandPanel) handleFlowInput(value string) (CommandPanel, tea.Cmd) {
	f := p.flow
	step := f.steps[f.current]

	// add フローの type が dynamic の場合は remote_host と remote_port をスキップ
	if f.name == "add" && step.key == "type" {
		f.values[step.key] = value
		if value == "dynamic" {
			// remote_host と remote_port をスキップ
			f.values["remote_host"] = ""
			f.values["remote_port"] = "0"
			f.current += 3 // type -> name へスキップ（remote_host, remote_port を飛ばす）
			if f.current >= len(f.steps) {
				return p.completeFlow()
			}
			p.AppendOutput(f.steps[f.current].label + ":")
			return p, nil
		}
	}

	// バリデーション
	if step.validate != nil {
		if err := step.validate(value); err != nil {
			p.AppendOutput(fmt.Sprintf("エラー: %s", err))
			p.AppendOutput(step.label + ":")
			return p, nil
		}
	}

	f.values[step.key] = value
	f.current++

	// 全ステップ完了
	if f.current >= len(f.steps) {
		return p.completeFlow()
	}

	// 次のステップを表示
	p.AppendOutput(f.steps[f.current].label + ":")
	return p, nil
}

// completeFlow はフロー完了時の処理を行う。
func (p CommandPanel) completeFlow() (CommandPanel, tea.Cmd) {
	f := p.flow
	values := make(map[string]string)
	for k, v := range f.values {
		values[k] = v
	}
	command := f.name

	p.flow = nil
	p.AppendOutput("実行中...")

	return p, func() tea.Msg {
		return tui.CommandExecuteMsg{Command: command, Values: values}
	}
}

// FocusInput はプロンプトにフォーカスを移す。
func (p *CommandPanel) FocusInput() tea.Cmd {
	return p.prompt.Focus()
}

// View はパネルを描画する（コンパクト: 出力行 + 入力行）。
func (p CommandPanel) View() string {
	contentWidth := p.width
	if contentWidth < 10 {
		contentWidth = 10
	}

	// セクションタイトル
	var title string
	if p.focused {
		title = tui.FocusIndicator + " " + tui.SectionTitleStyle.Render("Command")
	} else {
		title = "  " + tui.MutedStyle.Bold(true).Render("Command")
	}

	// 出力エリアの行数（タイトル1行 + 入力行1行を除く）
	outputLines := p.height - 2
	if outputLines < 1 {
		outputLines = 1
	}

	// 出力バッファから表示分を取得
	var displayOutput []string
	if len(p.output) > outputLines {
		displayOutput = p.output[len(p.output)-outputLines:]
	} else {
		displayOutput = p.output
	}

	// 出力を不足分の空行で埋める
	for len(displayOutput) < outputLines {
		displayOutput = append(displayOutput, "")
	}

	var rows []string
	rows = append(rows, title)
	for _, line := range displayOutput {
		// 出力行のプレフィックス装飾
		styled := p.styleOutputLine(line)
		rows = append(rows, "  "+styled)
	}
	rows = append(rows, "  "+p.prompt.View())

	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(contentWidth).Height(p.height).Render(content)
}

// styleOutputLine は出力行にスタイルを適用する。
func (p CommandPanel) styleOutputLine(line string) string {
	if line == "" {
		return ""
	}
	// エラー行
	if strings.Contains(line, "エラー") || strings.Contains(line, "Error") {
		return tui.ErrorStyle.Render("✗") + " " + tui.MutedStyle.Render(line)
	}
	// 成功行（「しました」「完了」等）
	if strings.Contains(line, "しました") || strings.Contains(line, "完了") || strings.Contains(line, "復元") {
		return tui.ActiveStyle.Render("✓") + " " + tui.MutedStyle.Render(line)
	}
	return tui.MutedStyle.Render(line)
}

// IsInFlow はフロー実行中かを返す。
func (p CommandPanel) IsInFlow() bool {
	return p.flow != nil
}

// --- ヘルパー関数 ---

func validateForwardType(s string) error {
	switch strings.ToLower(s) {
	case "local", "remote", "dynamic":
		return nil
	default:
		return fmt.Errorf("種別は local, remote, dynamic のいずれかを指定してください")
	}
}

func validatePort(s string) error {
	if s == "" {
		return nil // オプショナルな場合
	}
	port, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("数値を入力してください")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("ポート番号は 1-65535 の範囲で指定してください")
	}
	return nil
}

func ruleNames(rules []core.ForwardRule) []string {
	names := make([]string, len(rules))
	for i, r := range rules {
		names[i] = r.Name
	}
	return names
}

func sessionRuleNames(sessions []core.ForwardSession) []string {
	names := make([]string, len(sessions))
	for i, s := range sessions {
		names[i] = s.Rule.Name
	}
	return names
}

func filterSessionsByStatus(sessions []core.ForwardSession, status core.SessionStatus) []core.ForwardSession {
	var result []core.ForwardSession
	for _, s := range sessions {
		if s.Status == status {
			result = append(result, s)
		}
	}
	return result
}
