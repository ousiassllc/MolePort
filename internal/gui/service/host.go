package service

import "fmt"

// Hosts はホストの接続・切断（UC-30）。
type Hosts interface {
	// Connect は SSH 接続を非同期に開始する。成功時に当該ホストの auto_connect ルールを開始する（UC-4）。
	// 存在しないホストは core.NotFoundError、接続できない状態は ErrStateConflict、デーモン未起動は ErrNotReady。
	Connect(name string) error
	// Disconnect は当該ホストのアクティブな転送を停止してから SSH 接続を切断する。
	Disconnect(name string) error
}

// HostService は Core を直接操作する Hosts の実装。
type HostService struct {
	ops *operations
}

var _ Hosts = (*HostService)(nil)

// NewHostService は HostService を返す。cb は認証コールバックの提供元（credential.Broker）、
// act は DashboardService と共有する Activity、q / n は非同期結果の通知先。
func NewHostService(src BackendSource, cb CallbackProvider, act *Activity, q *NotificationQueue, n Notifier) *HostService {
	return &HostService{ops: newOperations(src, cb, act, q, n)}
}

// Connect は Hosts を実装する。認証待ちでブロックしないよう別ゴルーチンで Core を呼ぶ。
// 接続の失敗は Core が発行する SSHEventError（EventBridge がトーストにする）と Error 状態で反映される。
func (s *HostService) Connect(name string) error {
	b, err := s.ops.backend()
	if err != nil {
		return err
	}
	h, err := b.SSH.GetHost(name)
	if err != nil {
		return err
	}
	if s.ops.act.Connecting(name) || !hostConnectable(h.State) {
		return ErrStateConflict
	}
	if !s.ops.act.beginConnect(name) {
		return ErrStateConflict
	}
	go s.connect(b, name)
	return nil
}

func (s *HostService) connect(b Backend, name string) {
	err := b.SSH.ConnectWithCallback(name, s.ops.cb.Callback())
	s.ops.act.endConnect(name)
	s.ops.notifier.Notify(TopicHosts, TopicStatus)
	if err != nil {
		return
	}
	for _, rule := range b.Forward.GetRulesByHost(name) {
		if !rule.AutoConnect || s.forwardBusy(b, rule.Name) {
			continue
		}
		if s.ops.act.beginStart(rule.Name) {
			s.ops.runStart(b, rule)
		}
	}
}

// forwardBusy は rule が既に開始済み・開始処理中かを返す。
func (s *HostService) forwardBusy(b Backend, rule string) bool {
	if s.ops.act.Starting(rule) {
		return true
	}
	sess, err := b.Forward.GetSession(rule)
	return err == nil && !forwardStartable(sess.Status)
}

// Disconnect は Hosts を実装する。
func (s *HostService) Disconnect(name string) error {
	b, err := s.ops.backend()
	if err != nil {
		return err
	}
	h, err := b.SSH.GetHost(name)
	if err != nil {
		return err
	}
	if !hostDisconnectable(h.State) {
		return ErrStateConflict
	}
	for _, sess := range b.Forward.GetAllSessions() {
		if sess.Rule.Host != name || !forwardStoppable(sess.Status) {
			continue
		}
		if err := b.Forward.StopForward(sess.Rule.Name); err != nil {
			return fmt.Errorf("stop forward %q: %w", sess.Rule.Name, err)
		}
	}
	if err := b.SSH.Disconnect(name); err != nil {
		return err
	}
	return nil
}
