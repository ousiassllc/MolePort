package service

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/format"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// Dashboard はダッシュボードの ViewModel を組み立てる。
// 内蔵デーモンが起動していない間は空の ViewModel を返す。
type Dashboard interface {
	Header() HeaderView
	Hosts() []HostView
	Forwards() []ForwardView
	Status() StatusView
}

// DashboardService は Core のメモリ上の状態から ViewModel を組み立てる Dashboard の実装。
// ディスク・ネットワークアクセスは行わない。
type DashboardService struct {
	src BackendSource
	now func() time.Time
}

var _ Dashboard = (*DashboardService)(nil)

// NewDashboardService は src から Backend を取得して ViewModel を組み立てる DashboardService を返す。
func NewDashboardService(src BackendSource) *DashboardService {
	return &DashboardService{src: src, now: time.Now}
}

// Header はヘッダーの ViewModel を返す。
func (d *DashboardService) Header() HeaderView {
	b, ok := d.src.Backend()
	if !ok {
		return HeaderView{}
	}
	return HeaderView{Version: b.Info.Version, PID: b.Info.PID}
}

// Hosts はホスト行の ViewModel を返す。アクティブ転送数はセッションから数える。
func (d *DashboardService) Hosts() []HostView {
	b, ok := d.src.Backend()
	if !ok {
		return nil
	}
	hosts := b.SSH.GetHosts()
	active := activeForwardsByHost(b.Forward.GetAllSessions())
	views := make([]HostView, len(hosts))
	for i, h := range hosts {
		views[i] = newHostView(h, active[h.Name])
	}
	return views
}

// Forwards は転送行の ViewModel を返す。
func (d *DashboardService) Forwards() []ForwardView {
	b, ok := d.src.Backend()
	if !ok {
		return nil
	}
	sessions := b.Forward.GetAllSessions()
	now := d.now()
	views := make([]ForwardView, len(sessions))
	for i, s := range sessions {
		views[i] = newForwardView(s, now)
	}
	return views
}

// Status はステータスバーの ViewModel を返す。
func (d *DashboardService) Status() StatusView {
	b, ok := d.src.Backend()
	if !ok {
		return StatusView{}
	}
	hosts := b.SSH.GetHosts()
	sessions := b.Forward.GetAllSessions()
	v := StatusView{Hosts: len(hosts), Forwards: len(sessions)}
	for _, h := range hosts {
		if h.State == core.Connected {
			v.ConnectedHosts++
		}
	}
	for _, s := range sessions {
		if s.Status == core.Active {
			v.ActiveForwards++
		}
	}
	return v
}

func activeForwardsByHost(sessions []core.ForwardSession) map[string]int {
	counts := make(map[string]int)
	for _, s := range sessions {
		if s.Status == core.Active {
			counts[s.Rule.Host]++
		}
	}
	return counts
}

// newHostView は SSHHost から HostView を組み立てる。Actions は状態から決まる（HATEOAS）。
func newHostView(h core.SSHHost, activeCount int) HostView {
	state := hostStateKey(h.State)
	return HostView{
		Name:        h.Name,
		Address:     hostAddress(h),
		State:       state,
		StateLabel:  i18n.T("gui.status.host." + state),
		ActiveCount: activeCount,
		Actions: HostActions{
			CanConnect:    h.State == core.Disconnected || h.State == core.ConnectionError || h.State == core.PendingAuth,
			CanDisconnect: h.State == core.Connected || h.State == core.Reconnecting,
			CanAddForward: true,
		},
	}
}

// hostStateKey は接続状態を CSS・i18n のキーに使う文字列へ変換する。
func hostStateKey(s core.ConnectionState) string {
	switch s {
	case core.Connecting:
		return "connecting"
	case core.Connected:
		return "connected"
	case core.Reconnecting:
		return "reconnecting"
	case core.PendingAuth:
		return "pending_auth"
	case core.ConnectionError:
		return "error"
	default:
		return "disconnected"
	}
}

// hostAddress は "user@host:port" 形式の接続先表記を返す。
func hostAddress(h core.SSHHost) string {
	name := h.HostName
	if name == "" {
		name = h.Name
	}
	port := h.Port
	if port == 0 {
		port = 22
	}
	addr := net.JoinHostPort(name, strconv.Itoa(port))
	if h.User != "" {
		addr = h.User + "@" + addr
	}
	return addr
}

// newForwardView は ForwardSession から ForwardView を組み立てる。Actions は状態から決まる（HATEOAS）。
func newForwardView(s core.ForwardSession, now time.Time) ForwardView {
	status := sessionStatusKey(s.Status)
	v := ForwardView{
		Name:        s.Rule.Name,
		Host:        s.Rule.Host,
		Type:        s.Rule.Type.String(),
		Route:       forwardRoute(s.Rule),
		Status:      status,
		StatusLabel: i18n.T("gui.status.forward." + status),
		Sent:        format.Bytes(s.BytesSent),
		Received:    format.Bytes(s.BytesReceived),
		Reconnects:  s.ReconnectCount,
		LastError:   s.LastError,
		Actions: ForwardActions{
			CanStart:  s.Status == core.Stopped || s.Status == core.SessionError,
			CanStop:   s.Status == core.Starting || s.Status == core.Active || s.Status == core.SessionReconnecting,
			CanDelete: true,
		},
	}
	if s.Status == core.Active && !s.ConnectedAt.IsZero() {
		v.Uptime = format.Duration(now.Sub(s.ConnectedAt))
	}
	return v
}

// sessionStatusKey はセッション状態を CSS・i18n のキーに使う文字列へ変換する。
func sessionStatusKey(s core.SessionStatus) string {
	switch s {
	case core.Starting:
		return "starting"
	case core.Active:
		return "active"
	case core.SessionReconnecting:
		return "reconnecting"
	case core.SessionError:
		return "error"
	default:
		return "stopped"
	}
}

// forwardRoute は転送の経路表記を返す（local: ":LPORT → RHOST:RPORT"、
// remote: "RBIND:RPORT ← :LPORT"、dynamic: "SOCKS :LPORT"）。
func forwardRoute(r core.ForwardRule) string {
	switch r.Type {
	case core.Dynamic:
		return fmt.Sprintf("SOCKS :%d", r.LocalPort)
	case core.Remote:
		bind := r.RemoteBindAddr
		if bind == "" {
			bind = core.LocalhostAddr
		}
		return fmt.Sprintf("%s ← :%d", net.JoinHostPort(bind, strconv.Itoa(r.RemotePort)), r.LocalPort)
	default:
		return fmt.Sprintf(":%d → %s", r.LocalPort, net.JoinHostPort(r.RemoteHost, strconv.Itoa(r.RemotePort)))
	}
}
