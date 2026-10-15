//go:build gui

package tray

import (
	"reflect"
	"sync"

	"fyne.io/systray"
)

// Config はトレイの設定。メニュー内容は Build が現在の状態・言語で組み立てる（UC-33）。
type Config struct {
	// TemplateIcon は macOS のテンプレート画像。Icon は他 OS（および macOS の通常アイコン）で使う。
	TemplateIcon []byte
	Icon         []byte
	Tooltip      string
	// Build は現在のメニュー内容を返す。トレイの更新のたびに呼ばれる（BuildFrom を想定）。
	Build func() Menu
	// OnShow はトレイアイコンの左クリック / 「ウィンドウを表示」 / 「ウィンドウで全て表示…」で呼ばれる。
	OnShow func()
	// OnToggle は転送項目のクリックで呼ばれる（転送の開始 / 停止のトグル）。
	OnToggle func(name string)
	// OnStopAll は「すべての転送を停止」で呼ばれる。
	OnStopAll func()
	// OnQuit は「終了」で呼ばれる（Lifecycle.RequestQuit(true) を想定）。
	OnQuit func()
}

// Tray は起動済みのトレイ。
type Tray struct {
	cfg  Config
	end  func()
	done chan struct{}
	deb  *Debouncer

	mu       sync.Mutex // items / names / applied を保護する
	summary  *systray.MenuItem
	show     *systray.MenuItem
	forwards [MaxForwardItems]*systray.MenuItem
	names    [MaxForwardItems]string
	more     *systray.MenuItem
	stopAll  *systray.MenuItem
	quit     *systray.MenuItem
	applied  *Menu
}

// Start は systray を外部ループ（Wails 側のイベントループ）で起動し、トレイ利用可否を 1 回判定して返す。
// 利用不可でも Tray は返る（Refresh / Stop は常に呼べる）。
// systray の onReady に到達しない場合は ReadyTimeout 後に false を返す。
func Start(cfg Config) (*Tray, bool) {
	ready := make(chan struct{})
	t := &Tray{cfg: cfg, done: make(chan struct{})}
	t.deb = NewDebouncer(RefreshDelay, t.RefreshNow)

	start, end := systray.RunWithExternalLoop(func() {
		systray.SetTemplateIcon(cfg.TemplateIcon, cfg.Icon)
		systray.SetTooltip(cfg.Tooltip)
		systray.SetOnTapped(cfg.OnShow)
		t.buildItems()
		t.RefreshNow()
		close(ready)
	}, nil)
	t.end = end
	start()

	return t, Detect(ready, ReadyTimeout, hostCheck)
}

// buildItems はメニュー項目を 1 度だけ作る。以後は ResetMenu を使わず、既存項目の
// SetTitle / Check / Show / Hide だけで更新する（転送項目は最大 MaxForwardItems 件のプールを使い回す）。
func (t *Tray) buildItems() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.summary = systray.AddMenuItem("", "")
	t.summary.Disable()
	t.show = systray.AddMenuItem("", "")
	systray.AddSeparator()
	for i := range t.forwards {
		item := systray.AddMenuItemCheckbox("", "", false)
		item.Hide()
		t.forwards[i] = item
		t.listen(item.ClickedCh, func() { t.toggle(i) })
	}
	t.more = systray.AddMenuItem("", "")
	t.more.Hide()
	systray.AddSeparator()
	t.stopAll = systray.AddMenuItem("", "")
	t.quit = systray.AddMenuItem("", "")

	t.listen(t.show.ClickedCh, t.cfg.OnShow)
	t.listen(t.more.ClickedCh, t.cfg.OnShow)
	t.listen(t.stopAll.ClickedCh, t.cfg.OnStopAll)
	t.listen(t.quit.ClickedCh, t.cfg.OnQuit)
}

// listen はクリックのたびに fn を呼ぶ。fn が確認ダイアログ等でブロックしても他の項目に影響しない。
func (t *Tray) listen(ch <-chan struct{}, fn func()) {
	go func() {
		for {
			select {
			case <-ch:
				fn()
			case <-t.done:
				return
			}
		}
	}()
}

func (t *Tray) toggle(i int) {
	t.mu.Lock()
	name := t.names[i]
	t.mu.Unlock()
	if name == "" {
		return
	}
	t.cfg.OnToggle(name)
	// OS がチェック状態を先に反転する場合に備え、次の更新で全項目を再設定させる。
	t.mu.Lock()
	t.applied = nil
	t.mu.Unlock()
	t.Refresh()
}

// Refresh はメニュー更新を予約する（500ms デバウンス）。状態変化のたびに呼んでよい。
func (t *Tray) Refresh() { t.deb.Trigger() }

// RefreshNow はデバウンスせずにメニューを更新する。言語変更時に全項目のタイトルを再設定するために使う。
func (t *Tray) RefreshNow() {
	m := t.cfg.Build()

	t.mu.Lock()
	defer t.mu.Unlock()
	select {
	case <-t.done:
		return
	default:
	}
	if t.summary == nil || (t.applied != nil && reflect.DeepEqual(*t.applied, m)) {
		return
	}
	t.summary.SetTitle(m.Summary)
	t.show.SetTitle(m.Show)
	for i, item := range t.forwards {
		if i >= len(m.Forwards) {
			t.names[i] = ""
			item.Hide()
			continue
		}
		f := m.Forwards[i]
		t.names[i] = f.Name
		item.SetTitle(f.Title)
		if f.Checked {
			item.Check()
		} else {
			item.Uncheck()
		}
		item.Show()
	}
	if m.More != "" {
		t.more.SetTitle(m.More)
		t.more.Show()
	} else {
		t.more.Hide()
	}
	t.stopAll.SetTitle(m.StopAll)
	if m.StopAllEnabled {
		t.stopAll.Enable()
	} else {
		t.stopAll.Disable()
	}
	t.quit.SetTitle(m.Quit)
	t.applied = &m
}

// Stop はトレイを破棄する。複数回呼んでも安全。
func (t *Tray) Stop() {
	t.mu.Lock()
	select {
	case <-t.done:
		t.mu.Unlock()
		return
	default:
	}
	close(t.done)
	t.mu.Unlock()
	t.deb.Stop()
	t.end()
}
