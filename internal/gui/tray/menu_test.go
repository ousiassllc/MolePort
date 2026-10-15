package tray

import (
	"fmt"
	"testing"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/i18n"
)

func init() { _ = i18n.SetLang(i18n.LangEN) }

func fwdView(name string, canStart, canStop bool) service.ForwardView {
	return service.ForwardView{Name: name, Route: ":8080 → localhost:80", Actions: service.ForwardActions{CanStart: canStart, CanStop: canStop}}
}

func TestBuildMenu_Labels(t *testing.T) {
	m := BuildMenu(service.StatusView{ConnectedHosts: 2, ActiveForwards: 3}, nil)

	if want := "3 forward(s) active / 2 host(s) connected"; m.Summary != want {
		t.Errorf("Summary = %q, want %q", m.Summary, want)
	}
	if m.Show != "Show window" || m.Quit != "Quit" || m.StopAll != "Stop all forwards" {
		t.Errorf("labels = %q / %q / %q", m.Show, m.Quit, m.StopAll)
	}
	if len(m.Forwards) != 0 || m.More != "" || m.StopAllEnabled {
		t.Errorf("転送なし: %+v", m)
	}
}

func TestBuildMenu_ForwardItems(t *testing.T) {
	m := BuildMenu(service.StatusView{}, []service.ForwardView{
		fwdView("web", false, true),
		fwdView("db", true, false),
	})

	if len(m.Forwards) != 2 {
		t.Fatalf("Forwards = %+v", m.Forwards)
	}
	if got := m.Forwards[0]; got.Name != "web" || !got.Checked || got.Title != "web (:8080 → localhost:80)" {
		t.Errorf("稼働中の項目 = %+v", got)
	}
	if got := m.Forwards[1]; got.Name != "db" || got.Checked {
		t.Errorf("停止中の項目 = %+v", got)
	}
	if !m.StopAllEnabled {
		t.Error("停止可能な転送があれば全停止は有効")
	}
}

func TestBuildMenu_StopAllDisabledWithoutActive(t *testing.T) {
	m := BuildMenu(service.StatusView{}, []service.ForwardView{fwdView("db", true, false)})

	if m.StopAllEnabled {
		t.Error("停止可能な転送がなければ全停止は無効")
	}
}

func TestBuildMenu_CapsForwardItems(t *testing.T) {
	tests := []struct {
		count    int
		wantItem int
		wantMore bool
	}{
		{MaxForwardItems - 1, MaxForwardItems - 1, false},
		{MaxForwardItems, MaxForwardItems, false},
		{MaxForwardItems + 1, MaxForwardItems, true},
		{MaxForwardItems + 5, MaxForwardItems, true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.count), func(t *testing.T) {
			views := make([]service.ForwardView, tt.count)
			for i := range views {
				views[i] = fwdView(fmt.Sprintf("r%d", i), true, false)
			}
			// 21 件目以降のみ稼働中でも全停止の有効判定に含める。
			views[len(views)-1] = fwdView("last", false, true)

			m := BuildMenu(service.StatusView{}, views)

			if len(m.Forwards) != tt.wantItem {
				t.Errorf("項目数 = %d, want %d", len(m.Forwards), tt.wantItem)
			}
			if (m.More != "") != tt.wantMore {
				t.Errorf("More = %q, want 非空 = %v", m.More, tt.wantMore)
			}
			if !m.StopAllEnabled {
				t.Error("表示上限を超えた転送が稼働中でも全停止は有効")
			}
		})
	}
}

func TestBuildMenu_FollowsLanguage(t *testing.T) {
	t.Cleanup(func() { _ = i18n.SetLang(i18n.LangEN) })
	if err := i18n.SetLang(i18n.LangJA); err != nil {
		t.Fatal(err)
	}

	views := make([]service.ForwardView, MaxForwardItems+1)
	for i := range views {
		views[i] = fwdView(fmt.Sprintf("r%d", i), true, false)
	}

	m := BuildMenu(service.StatusView{ConnectedHosts: 1, ActiveForwards: 1}, views)

	if m.Quit != "終了" || m.Show != "ウィンドウを表示" || m.More != "ウィンドウで全て表示…" {
		t.Errorf("ja ラベル = %q / %q / %q", m.Quit, m.Show, m.More)
	}
	if want := "1 転送がアクティブ / 1 ホスト接続中"; m.Summary != want {
		t.Errorf("Summary = %q, want %q", m.Summary, want)
	}
}

type fakeSource struct {
	status   service.StatusView
	forwards []service.ForwardView
}

func (f fakeSource) Status() service.StatusView      { return f.status }
func (f fakeSource) Forwards() []service.ForwardView { return f.forwards }

func TestBuildFrom(t *testing.T) {
	m := BuildFrom(fakeSource{status: service.StatusView{ActiveForwards: 1}, forwards: []service.ForwardView{fwdView("web", false, true)}})

	if len(m.Forwards) != 1 || m.Forwards[0].Name != "web" {
		t.Errorf("Forwards = %+v", m.Forwards)
	}
}
