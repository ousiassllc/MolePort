package service

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/ousiassllc/moleport/internal/core"
)

var prodHost = core.SSHHost{Name: "prod", HostName: "10.0.0.1", Port: 22, User: "alice"}

func TestForwardService_AddValidation(t *testing.T) {
	useEnglish(t)
	tests := []struct {
		name  string
		in    ForwardFormInput
		field string
		want  string
	}{
		{"ホスト未選択", ForwardFormInput{Type: "local", LocalPort: "8080"}, "host", "Select a host."},
		{"未知のホスト", ForwardFormInput{Host: "nope", Type: "local", LocalPort: "8080"}, "host", `Host "nope" is not in the SSH config.`},
		{"不正な種別", ForwardFormInput{Host: "prod", Type: "x", LocalPort: "8080"}, "type", "The type must be local, remote or dynamic."},
		{"ポート未入力", ForwardFormInput{Host: "prod", Type: "local"}, "local_port", "Enter a port number."},
		{"ポート範囲外", ForwardFormInput{Host: "prod", Type: "local", LocalPort: "70000"}, "local_port", "The port must be an integer between 1 and 65535."},
		{"ポートが数値でない", ForwardFormInput{Host: "prod", Type: "dynamic", LocalPort: "abc"}, "local_port", "The port must be an integer between 1 and 65535."},
		{"リモートポート範囲外", ForwardFormInput{Host: "prod", Type: "remote", LocalPort: "8080", RemotePort: "0"}, "remote_port", "The port must be an integer between 1 and 65535."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newOpsEnv([]core.SSHHost{prodHost})
			name, errs, err := e.fwds.Add(tt.in)
			if err != nil || name != "" {
				t.Fatalf("Add = %q, %v", name, err)
			}
			if errs[tt.field] != tt.want {
				t.Errorf("errs[%s] = %q, want %q（errs=%v）", tt.field, errs[tt.field], tt.want, errs)
			}
			if len(e.fwd.GetRules()) != 0 || e.cfg.Saves() != 0 {
				t.Error("検証エラーなのにルールが追加・保存された")
			}
		})
	}
}

func TestForwardService_AddAppliesPlaceholdersPersistsAndStarts(t *testing.T) {
	tests := []struct {
		name string
		in   ForwardFormInput
		want core.ForwardRule
	}{
		{"local は remote_host / remote_port を補う", ForwardFormInput{Host: "prod", Type: "local", LocalPort: "8080"},
			core.ForwardRule{Name: "prod-local-8080", Host: "prod", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 8080}},
		{"remote はバインドアドレスも補う", ForwardFormInput{Host: "prod", Type: "remote", Name: "rev", LocalPort: "3000", RemotePort: "9000", AutoConnect: true},
			core.ForwardRule{Name: "rev", Host: "prod", Type: core.Remote, LocalPort: 3000, RemoteHost: "localhost", RemotePort: 9000, RemoteBindAddr: core.LocalhostAddr, AutoConnect: true}},
		{"dynamic はリモート項目を持たない", ForwardFormInput{Host: "prod", Type: "dynamic", LocalPort: "1080", RemoteHost: "ignored", RemotePort: "1"},
			core.ForwardRule{Name: "prod-dynamic-1080", Host: "prod", Type: core.Dynamic, LocalPort: 1080}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e := newOpsEnv([]core.SSHHost{prodHost})
				name, errs, err := e.fwds.Add(tt.in)
				if err != nil || len(errs) != 0 {
					t.Fatalf("Add: errs=%v err=%v", errs, err)
				}
				if name != tt.want.Name {
					t.Errorf("name = %q, want %q", name, tt.want.Name)
				}
				// core.PersistForwardRules で config.yaml に保存される。
				if saved := e.cfg.SavedRules(); !reflect.DeepEqual(saved, []core.ForwardRule{tt.want}) {
					t.Errorf("保存されたルール = %+v, want %+v", saved, tt.want)
				}
				synctest.Wait()
				if got := e.fwd.Started(); !reflect.DeepEqual(got, []string{tt.want.Name}) {
					t.Errorf("開始 = %v, want [%s]", got, tt.want.Name)
				}
			})
		})
	}
}

func TestForwardService_AddDuplicateNameIsFieldError(t *testing.T) {
	useEnglish(t)
	e := newOpsEnv([]core.SSHHost{prodHost}, session("web", "prod", core.Stopped))
	_, errs, err := e.fwds.Add(ForwardFormInput{Host: "prod", Type: "local", Name: "web", LocalPort: "8080"})
	if err != nil {
		t.Fatal(err)
	}
	if errs["name"] != `"web" already exists.` {
		t.Errorf("errs = %v", errs)
	}
}

func TestForwardService_AddPersistFailure(t *testing.T) {
	e := newOpsEnv([]core.SSHHost{prodHost})
	e.cfg.Err = errors.New("disk full")
	if _, _, err := e.fwds.Add(ForwardFormInput{Host: "prod", Type: "local", LocalPort: "8080"}); err == nil {
		t.Fatal("保存失敗がエラーにならない")
	}
	if len(e.fwd.Started()) != 0 {
		t.Error("保存失敗なのに開始された")
	}
}

func TestForwardService_StartStopConflicts(t *testing.T) {
	e := newOpsEnv([]core.SSHHost{prodHost}, session("on", "prod", core.Active), session("off", "prod", core.Stopped))

	if err := e.fwds.Start("on"); !errors.Is(err, ErrStateConflict) {
		t.Errorf("Active の Start = %v, want ErrStateConflict", err)
	}
	if err := e.fwds.Stop("off"); !errors.Is(err, ErrStateConflict) {
		t.Errorf("Stopped の Stop = %v, want ErrStateConflict", err)
	}
	var nf *core.NotFoundError
	if err := e.fwds.Start("none"); !errors.As(err, &nf) {
		t.Errorf("存在しないルールの Start = %v, want NotFoundError", err)
	}
	if err := e.fwds.Stop("on"); err != nil {
		t.Errorf("Active の Stop = %v", err)
	}
	if got := e.fwd.Stopped(); !reflect.DeepEqual(got, []string{"on"}) {
		t.Errorf("停止 = %v", got)
	}
}

func TestForwardService_StartIsAsyncAndShowsStarting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newOpsEnv([]core.SSHHost{prodHost}, session("web", "prod", core.Stopped))
		release := make(chan struct{})
		e.fwd.StartFn = func(name string, _ core.CredentialCallback) error {
			<-release // 認証待ちを模す
			e.fwd.SetStatus(name, core.Active)
			return nil
		}

		if err := e.fwds.Start("web"); err != nil {
			t.Fatalf("Start: %v", err)
		}
		synctest.Wait()
		// Core は Stopped のままでも、開始処理中は Starting として表示し、再度の開始は 409。
		if v := e.forwardView("web"); v.Status != "starting" || v.Actions.CanStart || !v.Actions.CanStop {
			t.Errorf("開始中の表示 = %+v", v)
		}
		if err := e.fwds.Start("web"); !errors.Is(err, ErrStateConflict) {
			t.Errorf("開始中の Start = %v, want ErrStateConflict", err)
		}

		close(release)
		synctest.Wait()
		if v := e.forwardView("web"); v.Status != "active" {
			t.Errorf("開始後の状態 = %q, want active", v.Status)
		}
		if !e.n.has(TopicForwards) {
			t.Error("開始完了で forwards トピックが通知されない")
		}
	})
}

func TestForwardService_StartFailureSuggestsFreePorts(t *testing.T) {
	useEnglish(t)
	synctest.Test(t, func(t *testing.T) {
		e := newOpsEnv([]core.SSHHost{prodHost}, session("web", "prod", core.Stopped))
		e.fwds.ops.portFree = func(p int) bool { return p != 8081 }
		e.fwd.StartFn = func(string, core.CredentialCallback) error {
			return errors.New("listen tcp 127.0.0.1:8080: bind: address already in use")
		}

		if err := e.fwds.Start("web"); err != nil {
			t.Fatalf("Start: %v", err)
		}
		synctest.Wait()

		wantMsg := "The port is already in use. Free ports you can use: 8082, 8083, 8084"
		v := e.forwardView("web")
		if v.Status != "error" || v.LastError != wantMsg || !v.Actions.CanStart {
			t.Errorf("失敗後の表示 = %+v", v)
		}
		toasts := e.q.Drain()
		if len(toasts) != 1 || toasts[0].Kind != ToastError || !strings.Contains(toasts[0].Message, wantMsg) {
			t.Errorf("トースト = %+v", toasts)
		}
		if !e.n.has(TopicToast) {
			t.Error("toast トピックが通知されない")
		}

		// 停止中のルールを再開始すると直近の失敗表示は消える。
		e.fwd.StartFn = nil
		if err := e.fwds.Start("web"); err != nil {
			t.Fatalf("再開始: %v", err)
		}
		synctest.Wait()
		if v := e.forwardView("web"); v.LastError != "" || v.Status != "active" {
			t.Errorf("再開始後の表示 = %+v", v)
		}
	})
}

func TestForwardService_DeletePersists(t *testing.T) {
	e := newOpsEnv([]core.SSHHost{prodHost}, session("a", "prod", core.Stopped), session("b", "prod", core.Stopped))
	if err := e.fwds.Delete("a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	saved := e.cfg.SavedRules()
	if len(saved) != 1 || saved[0].Name != "b" {
		t.Errorf("保存されたルール = %+v, want [b]", saved)
	}
	var nf *core.NotFoundError
	if err := e.fwds.Delete("a"); !errors.As(err, &nf) {
		t.Errorf("2 回目の Delete = %v, want NotFoundError", err)
	}
}

func TestForwardService_NotReady(t *testing.T) {
	s := NewForwardService(staticBackend{}, stubCallback{}, NewActivity(), NewNotificationQueue(), &syncNotifier{})
	if _, _, err := s.Add(ForwardFormInput{}); !errors.Is(err, ErrNotReady) {
		t.Errorf("Add = %v, want ErrNotReady", err)
	}
	for name, op := range map[string]func(string) error{"Start": s.Start, "Stop": s.Stop, "Delete": s.Delete} {
		if err := op("web"); !errors.Is(err, ErrNotReady) {
			t.Errorf("%s = %v, want ErrNotReady", name, err)
		}
	}
	if s.HostNames() != nil {
		t.Error("未起動なのにホスト名を返した")
	}
}

func TestForwardService_FormAndHostNames(t *testing.T) {
	e := newOpsEnv([]core.SSHHost{prodHost, {Name: "db"}})
	if got := e.fwds.HostNames(); !reflect.DeepEqual(got, []string{"prod", "db"}) {
		t.Errorf("HostNames = %v", got)
	}
	if got := e.fwds.Form("db"); got.Host != "db" || got.Type != "local" {
		t.Errorf("Form = %+v", got)
	}
}
