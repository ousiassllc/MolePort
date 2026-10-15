package shell

import "testing"

func TestDecideClose(t *testing.T) {
	tests := []struct {
		name          string
		quitting      bool
		trayAvailable bool
		want          CloseAction
		wantPrevent   bool
	}{
		{"終了処理中・トレイあり", true, true, CloseAllow, false},
		{"終了処理中・トレイなし", true, false, CloseAllow, false},
		{"トレイあり", false, true, CloseHide, true},
		{"トレイなし", false, false, CloseConfirmQuit, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecideClose(tt.quitting, tt.trayAvailable)
			if got != tt.want {
				t.Errorf("DecideClose(%v, %v) = %v, want %v", tt.quitting, tt.trayAvailable, got, tt.want)
			}
			if got.Prevent() != tt.wantPrevent {
				t.Errorf("Prevent() = %v, want %v", got.Prevent(), tt.wantPrevent)
			}
		})
	}
}

type fakeCloseHost struct {
	confirm bool
	hidden  int
	asked   int
	quits   int
}

func (f *fakeCloseHost) HideWindow() { f.hidden++ }
func (f *fakeCloseHost) ConfirmQuit() bool {
	f.asked++
	return f.confirm
}
func (f *fakeCloseHost) RequestQuit() { f.quits++ }

func TestCloseController_BeforeClose(t *testing.T) {
	tests := []struct {
		name        string
		quitting    bool
		tray        bool
		confirm     bool
		wantPrevent bool
		wantHidden  int
		wantAsked   int
		wantQuits   int
	}{
		{name: "終了処理中は素通し", quitting: true, tray: true, wantPrevent: false},
		{name: "トレイあり: 非表示にして中断", tray: true, wantPrevent: true, wantHidden: 1},
		{name: "トレイなし・承認: 終了を要求して中断", confirm: true, wantPrevent: true, wantAsked: 1, wantQuits: 1},
		{name: "トレイなし・拒否: 何もせず中断", wantPrevent: true, wantAsked: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := &fakeCloseHost{confirm: tt.confirm}
			c := NewCloseController(host,
				func() bool { return tt.quitting },
				func() bool { return tt.tray },
			)

			if got := c.BeforeClose(); got != tt.wantPrevent {
				t.Errorf("BeforeClose() = %v, want %v", got, tt.wantPrevent)
			}
			if host.hidden != tt.wantHidden || host.asked != tt.wantAsked || host.quits != tt.wantQuits {
				t.Errorf("hidden/asked/quits = %d/%d/%d, want %d/%d/%d",
					host.hidden, host.asked, host.quits, tt.wantHidden, tt.wantAsked, tt.wantQuits)
			}
		})
	}
}
