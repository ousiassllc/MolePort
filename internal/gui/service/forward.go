package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// formLevelError は FormErrors のうちフォーム全体に対するエラーのキー。
const formLevelError = "_form"

// Forwards は転送ルールの追加・開始・停止・削除（UC-28, UC-29）。
type Forwards interface {
	// HostNames は転送先に選べるホスト名（SSH config の Host エイリアス）を返す。デーモン未起動なら nil。
	HostNames() []string
	// Form は転送追加フォームの初期値を返す。host が空でなければ選択済みにする。
	Form(host string) ForwardFormInput
	// Add は入力を検証し、ルールを追加・保存して開始を非同期に始める。追加したルール名を返す。
	// 検証エラーはフィールド別の FormErrors で返す（その場合 error は nil）。
	Add(in ForwardFormInput) (name string, errs FormErrors, err error)
	// Start は転送を非同期に開始する。存在しないルールは core.NotFoundError、
	// 開始できない状態は ErrStateConflict、デーモン未起動は ErrNotReady。
	Start(name string) error
	// Stop は転送を停止する。
	Stop(name string) error
	// Delete は転送を停止してルールを削除し、保存する。
	Delete(name string) error
}

// ForwardService は Core を直接操作する Forwards の実装。
type ForwardService struct {
	ops *operations
}

var _ Forwards = (*ForwardService)(nil)

// NewForwardService は ForwardService を返す。引数の意味は NewHostService と同じ。
func NewForwardService(src BackendSource, cb CallbackProvider, act *Activity, q *NotificationQueue, n Notifier) *ForwardService {
	return &ForwardService{ops: newOperations(src, cb, act, q, n)}
}

// HostNames は Forwards を実装する。
func (s *ForwardService) HostNames() []string {
	b, err := s.ops.backend()
	if err != nil {
		return nil
	}
	hosts := b.SSH.GetHosts()
	names := make([]string, len(hosts))
	for i, h := range hosts {
		names[i] = h.Name
	}
	return names
}

// Form は Forwards を実装する。
func (s *ForwardService) Form(host string) ForwardFormInput {
	return ForwardFormInput{Host: host, Type: core.Local.String()}
}

// Add は Forwards を実装する。検証は core.ParseForwardType / core.ValidatePort を共用し、
// 空欄は placeholder の値を採用する（F-50, F-51）。
func (s *ForwardService) Add(in ForwardFormInput) (string, FormErrors, error) {
	b, err := s.ops.backend()
	if err != nil {
		return "", nil, err
	}
	rule, errs := s.buildRule(b, in)
	if len(errs) > 0 {
		return "", errs, nil
	}

	name, err := b.Forward.AddRule(rule)
	if err != nil {
		var exists *core.AlreadyExistsError
		if errors.As(err, &exists) {
			return "", FormErrors{"name": LocalizeError(err)}, nil
		}
		return "", FormErrors{formLevelError: LocalizeError(err)}, nil
	}
	rule.Name = name
	if err := core.PersistForwardRules(b.Config, b.Forward); err != nil {
		return name, nil, err
	}
	s.ops.startAsync(b, rule)
	return name, nil, nil
}

// buildRule は入力を検証して ForwardRule にする。エラーがあればフィールド別の FormErrors を返す。
func (s *ForwardService) buildRule(b Backend, in ForwardFormInput) (core.ForwardRule, FormErrors) {
	errs := FormErrors{}
	rule := core.ForwardRule{Host: strings.TrimSpace(in.Host), AutoConnect: in.AutoConnect}

	if rule.Host == "" {
		errs["host"] = i18n.T("gui.forward.error.host_required")
	} else if _, err := b.SSH.GetHost(rule.Host); err != nil {
		errs["host"] = i18n.T("gui.forward.error.host_unknown", map[string]any{"Name": rule.Host})
	}

	typ, err := core.ParseForwardType(strings.TrimSpace(in.Type))
	if err != nil {
		errs["type"] = i18n.T("gui.forward.error.type_invalid")
	}
	rule.Type = typ

	localPort := parsePort(in.LocalPort, "local_port", errs)
	rule.LocalPort = localPort

	if typ == core.Local || typ == core.Remote {
		rule.RemoteHost = strings.TrimSpace(in.RemoteHost)
		if rule.RemoteHost == "" {
			rule.RemoteHost = "localhost"
		}
		remotePort := strings.TrimSpace(in.RemotePort)
		if remotePort == "" {
			remotePort = strings.TrimSpace(in.LocalPort)
		}
		rule.RemotePort = parsePort(remotePort, "remote_port", errs)
	}
	if typ == core.Remote {
		rule.RemoteBindAddr = strings.TrimSpace(in.RemoteBindAddr)
		if rule.RemoteBindAddr == "" {
			rule.RemoteBindAddr = core.LocalhostAddr
		}
	}

	rule.Name = strings.TrimSpace(in.Name)
	if rule.Name == "" && len(errs) == 0 {
		rule.Name = fmt.Sprintf("%s-%s-%d", rule.Host, typ, localPort)
	}
	return rule, errs
}

// parsePort は s を 1〜65535 のポート番号として解釈する。不正なら errs[field] にメッセージを入れて 0 を返す。
func parsePort(s, field string, errs FormErrors) int {
	s = strings.TrimSpace(s)
	if s == "" {
		errs[field] = i18n.T("gui.forward.error.port_required")
		return 0
	}
	port, err := strconv.Atoi(s)
	if err != nil || core.ValidatePort(port) != nil {
		errs[field] = i18n.T("gui.forward.error.port_range", map[string]any{"Min": core.MinPort, "Max": core.MaxPort})
		return 0
	}
	return port
}

// Start は Forwards を実装する。認証待ちでブロックしないよう開始は別ゴルーチンで行う。
func (s *ForwardService) Start(name string) error {
	b, err := s.ops.backend()
	if err != nil {
		return err
	}
	sess, err := b.Forward.GetSession(name)
	if err != nil {
		return err
	}
	if status, _ := effectiveStatus(*sess, s.ops.act); !forwardStartable(status) {
		return ErrStateConflict
	}
	s.ops.startAsync(b, sess.Rule)
	return nil
}

// Stop は Forwards を実装する。
func (s *ForwardService) Stop(name string) error {
	b, err := s.ops.backend()
	if err != nil {
		return err
	}
	sess, err := b.Forward.GetSession(name)
	if err != nil {
		return err
	}
	if status, _ := effectiveStatus(*sess, s.ops.act); !forwardStoppable(status) {
		return ErrStateConflict
	}
	s.ops.act.clearFailure(name)
	return b.Forward.StopForward(name)
}

// Delete は Forwards を実装する。ルールの保存は core.PersistForwardRules を使う。
func (s *ForwardService) Delete(name string) error {
	b, err := s.ops.backend()
	if err != nil {
		return err
	}
	if err := b.Forward.DeleteRule(name); err != nil {
		return err
	}
	s.ops.act.clearFailure(name)
	return core.PersistForwardRules(b.Config, b.Forward)
}
