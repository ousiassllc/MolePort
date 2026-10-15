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
	act *Activity // 進行中の接続・転送開始を Core の状態に重ねる。nil なら重ねない
	now func() time.Time
}

var _ Dashboard = (*DashboardService)(nil)

// NewDashboardService は src から Backend を取得して ViewModel を組み立てる DashboardService を返す。
// act は HostService / Forwards と共有する Activity（nil 可）。
func NewDashboardService(src BackendSource, act *Activity) *DashboardService {
	return &DashboardService{src: src, act: act, now: time.Now}
}

// Header はヘッダーの ViewModel を返す。新バージョンは update_check 有効かつ新版ありのときのみ載せる。
func (d *DashboardService) Header() HeaderView {
	b, ok := d.src.Backend()
	if !ok {
		return HeaderView{}
	}
	v := HeaderView{Version: b.Info.Version, PID: b.Info.PID}
	if r, ok := AvailableUpdate(b); ok {
		v.UpdateAvailable, v.LatestVersion, v.ReleaseURL = true, r.LatestVersion, r.ReleaseURL
	}
	return v
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
		views[i] = newHostView(h, active[h.Name], d.act.Connecting(h.Name))
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
		views[i] = newForwardView(s, now, d.act)
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
// connecting が true（非同期の接続を進行中）で Core がまだ接続処理に入っていない場合は「接続中」として扱う。
func newHostView(h core.SSHHost, activeCount int, connecting bool) HostView {
	state := h.State
	if connecting && hostConnectable(state) {
		state = core.Connecting
	}
	key := hostStateKey(state)
	return HostView{
		Name:        h.Name,
		Address:     hostAddress(h),
		State:       key,
		StateLabel:  i18n.T("gui.status.host." + key),
		ActiveCount: activeCount,
		Actions: HostActions{
			CanConnect:    hostConnectable(state),
			CanDisconnect: hostDisconnectable(state),
			CanAddForward: true,
		},
	}
}

// hostConnectable は接続操作が可能な状態（Disconnected / Error / PendingAuth）かを返す。
func hostConnectable(s core.ConnectionState) bool {
	return s == core.Disconnected || s == core.ConnectionError || s == core.PendingAuth
}

// hostDisconnectable は切断操作が可能な状態（Connected / Reconnecting）かを返す。
func hostDisconnectable(s core.ConnectionState) bool {
	return s == core.Connected || s == core.Reconnecting
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
// act は進行中の開始（Starting）と直近の開始失敗（Error）を Core の Stopped に重ねる。
func newForwardView(s core.ForwardSession, now time.Time, act *Activity) ForwardView {
	status, lastError := effectiveStatus(s, act)
	key := sessionStatusKey(status)
	v := ForwardView{
		Name:        s.Rule.Name,
		Host:        s.Rule.Host,
		Type:        s.Rule.Type.String(),
		Route:       forwardRoute(s.Rule),
		Status:      key,
		StatusLabel: i18n.T("gui.status.forward." + key),
		Sent:        format.Bytes(s.BytesSent),
		Received:    format.Bytes(s.BytesReceived),
		Reconnects:  s.ReconnectCount,
		LastError:   lastError,
		Actions: ForwardActions{
			CanStart:  forwardStartable(status),
			CanStop:   forwardStoppable(status),
			CanDelete: true,
		},
	}
	if status == core.Active && !s.ConnectedAt.IsZero() {
		v.Uptime = format.Duration(now.Sub(s.ConnectedAt))
	}
	return v
}

// effectiveStatus は Core のセッション状態に Activity（進行中の開始・直近の開始失敗）を重ねた状態と
// エラー詳細を返す。表示（Actions）と操作の可否判定（409）で同じ値を使う。
func effectiveStatus(s core.ForwardSession, act *Activity) (core.SessionStatus, string) {
	if s.Status != core.Stopped {
		return s.Status, s.LastError
	}
	if act.Starting(s.Rule.Name) {
		return core.Starting, s.LastError
	}
	if msg := act.Failure(s.Rule.Name); msg != "" {
		return core.SessionError, msg
	}
	return s.Status, s.LastError
}

// forwardStartable は開始操作が可能な状態（Stopped / Error）かを返す。
func forwardStartable(s core.SessionStatus) bool { return s == core.Stopped || s == core.SessionError }

// forwardStoppable は停止操作が可能な状態（Starting / Active / Reconnecting）かを返す。
func forwardStoppable(s core.SessionStatus) bool {
	return s == core.Starting || s == core.Active || s == core.SessionReconnecting
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
