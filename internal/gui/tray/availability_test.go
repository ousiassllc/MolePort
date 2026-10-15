package tray

import (
	"testing"
	"time"
)

func TestAvailable(t *testing.T) {
	tests := []struct {
		name                                    string
		readyReached, hostRequired, hostPresent bool
		want                                    bool
	}{
		{"onReady 未到達", false, false, false, false},
		{"onReady 未到達 (ホストあり)", false, true, true, false},
		{"ホスト不要の OS", true, false, false, true},
		{"Linux でホストあり", true, true, true, true},
		{"Linux でホスト不在", true, true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Available(tt.readyReached, tt.hostRequired, tt.hostPresent); got != tt.want {
				t.Errorf("Available(%v, %v, %v) = %v, want %v", tt.readyReached, tt.hostRequired, tt.hostPresent, got, tt.want)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	present := func() (bool, bool) { return true, true }
	absent := func() (bool, bool) { return true, false }
	notRequired := func() (bool, bool) { return false, false }
	neverCalled := func() (bool, bool) {
		t.Error("onReady 未到達ではホスト確認を呼ばない")
		return true, true
	}

	closed := make(chan struct{})
	close(closed)

	t.Run("ready 到達 + ホストあり", func(t *testing.T) {
		if !Detect(closed, time.Second, present) {
			t.Error("Detect = false, want true")
		}
	})
	t.Run("ready 到達 + ホスト不在", func(t *testing.T) {
		if Detect(closed, time.Second, absent) {
			t.Error("Detect = true, want false")
		}
	})
	t.Run("ready 到達 + ホスト不要", func(t *testing.T) {
		if !Detect(closed, time.Second, notRequired) {
			t.Error("Detect = false, want true")
		}
	})
	t.Run("タイムアウト", func(t *testing.T) {
		if Detect(make(chan struct{}), 10*time.Millisecond, neverCalled) {
			t.Error("Detect = true, want false (onReady 未到達)")
		}
	})
	t.Run("タイムアウト前に遅れて到達", func(t *testing.T) {
		ready := make(chan struct{})
		go func() {
			time.Sleep(10 * time.Millisecond)
			close(ready)
		}()
		if !Detect(ready, time.Second, present) {
			t.Error("Detect = false, want true")
		}
	})
}
