package update

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNormalizeInterval(t *testing.T) {
	tests := []struct {
		name        string
		in          time.Duration
		want        time.Duration
		wantChanged bool
	}{
		{"zero", 0, DefaultInterval, true},
		{"negative", -time.Hour, DefaultInterval, true},
		{"below minimum", 30 * time.Minute, MinInterval, true},
		{"minimum", time.Hour, time.Hour, false},
		{"default", 24 * time.Hour, 24 * time.Hour, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := NormalizeInterval(tt.in)
			if got != tt.want || changed != tt.wantChanged {
				t.Errorf("NormalizeInterval(%v) = %v, %v; want %v, %v", tt.in, got, changed, tt.want, tt.wantChanged)
			}
		})
	}
}

// interval が 0 以下 / 1h 未満でも loop が time.NewTicker で panic しない。
func TestVersionChecker_LoopSurvivesInvalidInterval(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Hour, time.Millisecond} {
		t.Run(interval.String(), func(t *testing.T) {
			srv := newTestServer("v0.2.0", "https://example.com", http.StatusOK)
			defer srv.Close()
			vc := newTestChecker(srv.URL, "v0.1.0", true)
			vc.interval = interval

			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan struct{})
			go func() {
				defer close(done)
				vc.loop(ctx, 0)
			}()

			// 初回チェック完了（キャッシュ更新）の後に Ticker が作られる。
			deadline := time.Now().Add(5 * time.Second)
			for !vc.UpdateAvailable() {
				if time.Now().After(deadline) {
					t.Fatal("初回チェックが行われない")
				}
				time.Sleep(time.Millisecond)
			}
			cancel()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("loop が終了しない")
			}
		})
	}
}
