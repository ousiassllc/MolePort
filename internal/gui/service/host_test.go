package service

import (
	"errors"
	"reflect"
	"testing"
	"testing/synctest"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestHostService_ConnectIsAsyncAndStartsAutoConnectRules(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		auto := session("auto", "prod", core.Stopped)
		auto.Rule.AutoConnect = true
		e := newOpsEnv([]core.SSHHost{prodHost}, auto, session("manual", "prod", core.Stopped))
		release := make(chan struct{})
		e.ssh.ConnectFn = func(name string, cb core.CredentialCallback) error {
			if cb == nil {
				t.Error("認証コールバックが渡されない")
			}
			<-release // 認証待ちを模す
			e.ssh.SetState(name, core.Connected)
			return nil
		}

		if err := e.hosts.Connect("prod"); err != nil {
			t.Fatalf("Connect: %v", err)
		}
		synctest.Wait()
		// Core が接続処理に入る前でも「接続中」を返し、再度の接続は 409。
		if v := e.hostView("prod"); v.State != "connecting" || v.Actions.CanConnect || v.Actions.CanDisconnect {
			t.Errorf("接続中の表示 = %+v", v)
		}
		if err := e.hosts.Connect("prod"); !errors.Is(err, ErrStateConflict) {
			t.Errorf("接続中の Connect = %v, want ErrStateConflict", err)
		}

		close(release)
		synctest.Wait()
		if v := e.hostView("prod"); v.State != "connected" || !v.Actions.CanDisconnect {
			t.Errorf("接続後の表示 = %+v", v)
		}
		if got := e.fwd.Started(); !reflect.DeepEqual(got, []string{"auto"}) {
			t.Errorf("開始された転送 = %v, want [auto]（auto_connect のみ）", got)
		}
		if !e.n.has(TopicHosts) {
			t.Error("接続完了で hosts トピックが通知されない")
		}
	})
}

func TestHostService_ConnectFailureStartsNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		auto := session("auto", "prod", core.Stopped)
		auto.Rule.AutoConnect = true
		e := newOpsEnv([]core.SSHHost{prodHost}, auto)
		e.ssh.ConnectFn = func(name string, _ core.CredentialCallback) error {
			e.ssh.SetState(name, core.ConnectionError)
			return errors.New("auth failed")
		}

		if err := e.hosts.Connect("prod"); err != nil {
			t.Fatalf("Connect: %v", err)
		}
		synctest.Wait()
		if len(e.fwd.Started()) != 0 {
			t.Error("接続失敗なのに転送が開始された")
		}
		// 失敗は Error 状態として表示され、再接続できる。
		if v := e.hostView("prod"); v.State != "error" || !v.Actions.CanConnect {
			t.Errorf("失敗後の表示 = %+v", v)
		}
	})
}

func TestHostService_ConnectConflicts(t *testing.T) {
	connected := prodHost
	connected.State = core.Connected
	e := newOpsEnv([]core.SSHHost{connected})

	if err := e.hosts.Connect("prod"); !errors.Is(err, ErrStateConflict) {
		t.Errorf("接続済みの Connect = %v, want ErrStateConflict", err)
	}
	var nf *core.NotFoundError
	if err := e.hosts.Connect("none"); !errors.As(err, &nf) {
		t.Errorf("存在しないホスト = %v, want NotFoundError", err)
	}
	if len(e.ssh.Connected()) != 0 {
		t.Error("409 / 404 なのに Core を呼んだ")
	}
}

func TestHostService_DisconnectStopsHostForwards(t *testing.T) {
	connected := prodHost
	connected.State = core.Connected
	other := core.SSHHost{Name: "db", State: core.Connected}
	e := newOpsEnv([]core.SSHHost{connected, other},
		session("web", "prod", core.Active),
		session("idle", "prod", core.Stopped),
		session("db-web", "db", core.Active),
	)

	if err := e.hosts.Disconnect("prod"); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if got := e.fwd.Stopped(); !reflect.DeepEqual(got, []string{"web"}) {
		t.Errorf("停止した転送 = %v, want [web]", got)
	}
	if v := e.hostView("prod"); v.State != "disconnected" {
		t.Errorf("切断後の状態 = %q", v.State)
	}
	if err := e.hosts.Disconnect("prod"); !errors.Is(err, ErrStateConflict) {
		t.Errorf("切断済みの Disconnect = %v, want ErrStateConflict", err)
	}
}

func TestHostService_NotReady(t *testing.T) {
	s := NewHostService(staticBackend{}, stubCallback{}, NewActivity(), NewNotificationQueue(), &syncNotifier{})
	if err := s.Connect("prod"); !errors.Is(err, ErrNotReady) {
		t.Errorf("Connect = %v, want ErrNotReady", err)
	}
	if err := s.Disconnect("prod"); !errors.Is(err, ErrNotReady) {
		t.Errorf("Disconnect = %v, want ErrNotReady", err)
	}
}
