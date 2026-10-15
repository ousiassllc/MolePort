package service

import (
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/gui/service/servicetest"
	"github.com/ousiassllc/moleport/internal/i18n"
)

func useEnglish(t *testing.T) {
	t.Helper()
	prev := i18n.CurrentLang()
	t.Cleanup(func() { _ = i18n.SetLang(prev) })
	if err := i18n.SetLang(i18n.LangEN); err != nil {
		t.Fatalf("SetLang: %v", err)
	}
}

func newDashboard(ssh core.SSHManager, fwd core.ForwardManager, now time.Time) *DashboardService {
	d := NewDashboardService(staticBackend{ok: true, b: Backend{SSH: ssh, Forward: fwd, Info: Info{Version: "1.2.3", PID: 99}}}, NewActivity())
	d.now = func() time.Time { return now }
	return d
}

func TestDashboard_NotReadyReturnsEmpty(t *testing.T) {
	d := NewDashboardService(staticBackend{}, nil)

	if got := d.Header(); got != (HeaderView{}) {
		t.Errorf("Header() = %+v, want zero", got)
	}
	if got := d.Hosts(); got != nil {
		t.Errorf("Hosts() = %v, want nil", got)
	}
	if got := d.Forwards(); got != nil {
		t.Errorf("Forwards() = %v, want nil", got)
	}
	if got := d.Status(); got != (StatusView{}) {
		t.Errorf("Status() = %+v, want zero", got)
	}
}

func TestDashboard_Header(t *testing.T) {
	d := newDashboard(servicetest.NewSSH(), servicetest.NewForward(), time.Now())
	if got, want := d.Header(), (HeaderView{Version: "1.2.3", PID: 99}); got != want {
		t.Errorf("Header() = %+v, want %+v", got, want)
	}
}

func TestDashboard_HeaderUpdateBadge(t *testing.T) {
	const release = ReleaseURLPrefix + "tag/v2.0.0"
	newer := &servicetest.Version{OK: true, Result: core.VersionCheckResult{UpdateAvailable: true, LatestVersion: "v2.0.0", ReleaseURL: release}}
	tests := []struct {
		name    string
		enabled bool
		ver     VersionSource
		want    bool
	}{
		{"有効かつ新版あり", true, newer, true},
		{"update_check 無効", false, newer, false},
		{"新版なし", true, &servicetest.Version{OK: true}, false},
		{"VersionChecker なし", true, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := core.DefaultConfig()
			cfg.UpdateCheck.Enabled = tt.enabled
			b := Backend{Config: servicetest.NewConfig(cfg), Version: tt.ver, Info: Info{Version: "1.0.0"}}
			got := NewDashboardService(staticBackend{ok: true, b: b}, nil).Header()
			if got.UpdateAvailable != tt.want {
				t.Fatalf("UpdateAvailable = %v, want %v", got.UpdateAvailable, tt.want)
			}
			if tt.want && (got.LatestVersion != "v2.0.0" || got.ReleaseURL != release) {
				t.Errorf("Header = %+v", got)
			}
		})
	}
}

func TestDashboard_HostsActionsByState(t *testing.T) {
	useEnglish(t)
	hosts := []core.SSHHost{
		{Name: "a", HostName: "a.example.com", Port: 2222, User: "alice", State: core.Disconnected},
		{Name: "b", State: core.Connecting},
		{Name: "c", State: core.Connected},
		{Name: "d", State: core.Reconnecting},
		{Name: "e", State: core.PendingAuth},
		{Name: "f", State: core.ConnectionError},
	}
	sessions := []core.ForwardSession{
		{Rule: core.ForwardRule{Name: "r1", Host: "c"}, Status: core.Active},
		{Rule: core.ForwardRule{Name: "r2", Host: "c"}, Status: core.Active},
		{Rule: core.ForwardRule{Name: "r3", Host: "c"}, Status: core.Stopped},
	}
	got := newDashboard(servicetest.NewSSH(hosts...), servicetest.NewForward(sessions...), time.Now()).Hosts()

	tests := []struct {
		name                       string
		state, label, addr         string
		connect, disconnect, count int
	}{
		{"a", "disconnected", "Disconnected", "alice@a.example.com:2222", 1, 0, 0},
		{"b", "connecting", "Connecting...", "b:22", 0, 0, 0},
		{"c", "connected", "Connected", "c:22", 0, 1, 2},
		{"d", "reconnecting", "Reconnecting", "d:22", 0, 1, 0},
		{"e", "pending_auth", "Pending auth", "e:22", 1, 0, 0},
		{"f", "error", "Error", "f:22", 1, 0, 0},
	}
	if len(got) != len(tests) {
		t.Fatalf("len(Hosts()) = %d, want %d", len(got), len(tests))
	}
	for i, tt := range tests {
		v := got[i]
		if v.Name != tt.name || v.State != tt.state || v.StateLabel != tt.label || v.Address != tt.addr || v.ActiveCount != tt.count {
			t.Errorf("host %s = %+v", tt.name, v)
		}
		if v.Actions.CanConnect != (tt.connect == 1) || v.Actions.CanDisconnect != (tt.disconnect == 1) || !v.Actions.CanAddForward {
			t.Errorf("host %s actions = %+v", tt.name, v.Actions)
		}
	}
}

func TestDashboard_ForwardsViewAndActions(t *testing.T) {
	useEnglish(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	sessions := []core.ForwardSession{
		{
			Rule:   core.ForwardRule{Name: "web", Host: "h", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80},
			Status: core.Active, ConnectedAt: now.Add(-(2*time.Hour + 15*time.Minute)),
			BytesSent: 1536, BytesReceived: 2 * 1024 * 1024, ReconnectCount: 3,
		},
		{Rule: core.ForwardRule{Name: "rev", Host: "h", Type: core.Remote, LocalPort: 3000, RemotePort: 9000}, Status: core.Starting},
		{Rule: core.ForwardRule{Name: "socks", Host: "h", Type: core.Dynamic, LocalPort: 1080}, Status: core.Stopped},
		{Rule: core.ForwardRule{Name: "bad", Host: "h", Type: core.Local, LocalPort: 1, RemoteHost: "x", RemotePort: 2}, Status: core.SessionError, LastError: "port in use"},
		{Rule: core.ForwardRule{Name: "re", Host: "h", Type: core.Remote, LocalPort: 1, RemotePort: 2, RemoteBindAddr: "0.0.0.0"}, Status: core.SessionReconnecting},
	}
	got := newDashboard(servicetest.NewSSH(), servicetest.NewForward(sessions...), now).Forwards()

	tests := []struct {
		name, typ, route, status, label, uptime, sent, recv string
		start, stop                                         bool
	}{
		{"web", "local", ":8080 → localhost:80", "active", "Active", "2h 15m", "1.5KB", "2.0MB", false, true},
		{"rev", "remote", "127.0.0.1:9000 ← :3000", "starting", "Starting", "", "0B", "0B", false, true},
		{"socks", "dynamic", "SOCKS :1080", "stopped", "Stopped", "", "0B", "0B", true, false},
		{"bad", "local", ":1 → x:2", "error", "Error", "", "0B", "0B", true, false},
		{"re", "remote", "0.0.0.0:2 ← :1", "reconnecting", "Reconnecting", "", "0B", "0B", false, true},
	}
	if len(got) != len(tests) {
		t.Fatalf("len(Forwards()) = %d, want %d", len(got), len(tests))
	}
	for i, tt := range tests {
		v := got[i]
		if v.Name != tt.name || v.Type != tt.typ || v.Route != tt.route || v.Status != tt.status ||
			v.StatusLabel != tt.label || v.Uptime != tt.uptime || v.Sent != tt.sent || v.Received != tt.recv {
			t.Errorf("forward %s = %+v", tt.name, v)
		}
		if v.Actions.CanStart != tt.start || v.Actions.CanStop != tt.stop || !v.Actions.CanDelete {
			t.Errorf("forward %s actions = %+v", tt.name, v.Actions)
		}
	}
	if got[0].Reconnects != 3 {
		t.Errorf("Reconnects = %d, want 3", got[0].Reconnects)
	}
	if got[3].LastError != "port in use" {
		t.Errorf("LastError = %q", got[3].LastError)
	}
}

func TestDashboard_Status(t *testing.T) {
	hosts := []core.SSHHost{{Name: "a", State: core.Connected}, {Name: "b", State: core.Disconnected}, {Name: "c", State: core.Connected}}
	sessions := []core.ForwardSession{
		{Rule: core.ForwardRule{Name: "x"}, Status: core.Active},
		{Rule: core.ForwardRule{Name: "y"}, Status: core.Stopped},
	}
	got := newDashboard(servicetest.NewSSH(hosts...), servicetest.NewForward(sessions...), time.Now()).Status()

	if want := (StatusView{Hosts: 3, ConnectedHosts: 2, Forwards: 2, ActiveForwards: 1}); got != want {
		t.Errorf("Status() = %+v, want %+v", got, want)
	}
}

func TestDashboard_IPv6AddressIsBracketed(t *testing.T) {
	got := newDashboard(servicetest.NewSSH(core.SSHHost{Name: "v6", HostName: "::1", Port: 22}), servicetest.NewForward(), time.Now()).Hosts()
	if got[0].Address != "[::1]:22" {
		t.Errorf("Address = %q, want [::1]:22", got[0].Address)
	}
}
