//go:build gui

package tray

import "fyne.io/systray"

// Config はトレイの設定。ラベルは呼び出し側が i18n で解決して渡す。
type Config struct {
	// TemplateIcon は macOS のテンプレート画像。Icon は他 OS（および macOS の通常アイコン）で使う。
	TemplateIcon []byte
	Icon         []byte
	Tooltip      string
	ShowLabel    string
	QuitLabel    string
	// OnShow はトレイアイコンのクリック / 「表示」メニューで呼ばれる。
	OnShow func()
	// OnQuit は「終了」メニューで呼ばれる（Lifecycle.RequestQuit(true) を想定）。
	OnQuit func()
}

// Tray は起動済みのトレイ。
type Tray struct {
	end  func()
	done chan struct{}
}

// Start は systray を外部ループ（Wails 側のイベントループ）で起動し、トレイ利用可否を 1 回判定して返す。
// 利用不可でも Tray は返る（Stop は常に呼べる）。
// systray の onReady に到達しない場合は ReadyTimeout 後に false を返す。
func Start(cfg Config) (*Tray, bool) {
	ready := make(chan struct{})
	t := &Tray{done: make(chan struct{})}

	start, end := systray.RunWithExternalLoop(func() {
		systray.SetTemplateIcon(cfg.TemplateIcon, cfg.Icon)
		systray.SetTooltip(cfg.Tooltip)
		systray.SetOnTapped(cfg.OnShow)
		show := systray.AddMenuItem(cfg.ShowLabel, "")
		systray.AddSeparator()
		quit := systray.AddMenuItem(cfg.QuitLabel, "")
		go t.dispatch(show.ClickedCh, quit.ClickedCh, cfg)
		close(ready)
	}, nil)
	t.end = end
	start()

	return t, Detect(ready, ReadyTimeout, hostCheck)
}

func (t *Tray) dispatch(show, quit <-chan struct{}, cfg Config) {
	for {
		select {
		case <-show:
			cfg.OnShow()
		case <-quit:
			cfg.OnQuit()
		case <-t.done:
			return
		}
	}
}

// Stop はトレイを破棄する。複数回呼んでも安全。
func (t *Tray) Stop() {
	select {
	case <-t.done:
		return
	default:
	}
	close(t.done)
	t.end()
}
