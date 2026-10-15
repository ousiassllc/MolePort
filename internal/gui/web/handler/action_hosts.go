package handler

import (
	"net/http"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/web/view/organisms"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// HostActions はホストのアクション（/actions/hosts/{name}/...）。成功時は更新後のホストパネルを返す。
type HostActions struct {
	hosts service.Hosts
	dash  service.Dashboard
}

// NewHostActions は HostActions を返す。
func NewHostActions(hosts service.Hosts, dash service.Dashboard) *HostActions {
	return &HostActions{hosts: hosts, dash: dash}
}

// Connect は POST /actions/hosts/{name}/connect。接続は非同期で、パネルには「接続中」を即座に返す。
func (a *HostActions) Connect(w http.ResponseWriter, r *http.Request) {
	a.transition(w, r, a.hosts.Connect)
}

// Disconnect は POST /actions/hosts/{name}/disconnect。
func (a *HostActions) Disconnect(w http.ResponseWriter, r *http.Request) {
	a.transition(w, r, a.hosts.Disconnect)
}

func (a *HostActions) transition(w http.ResponseWriter, r *http.Request, op func(string) error) {
	name := r.PathValue("name")
	if err := op(name); err != nil {
		problem(w, r, err, eventHosts, i18n.T("gui.host.toast.conflict", map[string]any{"Name": name}))
		return
	}
	render(w, r, http.StatusOK, organisms.HostPanel(a.dash.Hosts(), true))
}
