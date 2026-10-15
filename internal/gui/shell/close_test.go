package shell

import (
	"fmt"
	"testing"
)

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

type fakeQuitController struct {
	quitting bool
	active   int
	requests []bool
}

func (f *fakeQuitController) Quitting() bool      { return f.quitting }
func (f *fakeQuitController) ActiveForwards() int { return f.active }
func (f *fakeQuitController) RequestQuit(c bool)  { f.requests = append(f.requests, c) }

func TestCloseController_BeforeClose(t *testing.T) {
	tests := []struct {
		name         string
		quitting     bool
		tray         bool
		confirm      bool
		active       int
		wantPrevent  bool
		wantHidden   int
		wantHidden2  int // onHidden の呼び出し回数
		wantAsked    []int
		wantRequests []bool
	}{
		{name: "終了処理中は素通し", quitting: true, tray: true, wantPrevent: false},
		{name: "トレイあり: 非表示にして中断", tray: true, wantPrevent: true, wantHidden: 1, wantHidden2: 1},
		{name: "トレイなし・承認: 確認なしの終了を要求して中断", confirm: true, active: 2, wantPrevent: true, wantAsked: []int{2}, wantRequests: []bool{false}},
		{name: "トレイなし・拒否: 何もせず中断", active: 1, wantPrevent: true, wantAsked: []int{1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			win := &fakeWindow{}
			dlg := &fakeDialog{confirm: tt.confirm}
			life := &fakeQuitController{quitting: tt.quitting, active: tt.active}
			hidden := 0
			c := NewCloseController(win, dlg, life, func() bool { return tt.tray }, func() { hidden++ })

			if got := c.BeforeClose(); got != tt.wantPrevent {
				t.Errorf("BeforeClose() = %v, want %v", got, tt.wantPrevent)
			}
			if win.hides != tt.wantHidden || hidden != tt.wantHidden2 {
				t.Errorf("hides/onHidden = %d/%d, want %d/%d", win.hides, hidden, tt.wantHidden, tt.wantHidden2)
			}
			if fmt.Sprint(dlg.closeAsks) != fmt.Sprint(tt.wantAsked) {
				t.Errorf("ConfirmClose = %v, want %v", dlg.closeAsks, tt.wantAsked)
			}
			if fmt.Sprint(life.requests) != fmt.Sprint(tt.wantRequests) {
				t.Errorf("RequestQuit = %v, want %v", life.requests, tt.wantRequests)
			}
		})
	}
}
