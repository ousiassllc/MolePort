package shell

// CloseAction はウィンドウの閉じる操作（OnBeforeClose）に対する決定。
type CloseAction int

const (
	// CloseAllow はウィンドウをそのまま閉じる（終了処理中）。
	CloseAllow CloseAction = iota
	// CloseHide はウィンドウを非表示にしてトレイに常駐する。
	CloseHide
	// CloseConfirmQuit は終了確認ダイアログを出し、承認されたら終了する（トレイ利用不可）。
	CloseConfirmQuit
)

// Prevent は OnBeforeClose が返す prevent の値。Allow 以外は閉じる操作を中断する。
func (a CloseAction) Prevent() bool { return a != CloseAllow }

// DecideClose は docs/architecture/gui.md「ウィンドウ閉じる操作の扱い」の決定表。
//
//	quitting  trayAvailable  結果
//	true      *              CloseAllow       (prevent=false)
//	false     true           CloseHide        (prevent=true)
//	false     false          CloseConfirmQuit (prevent=true)
//
// トレイがない間に「非表示」にすると復帰手段が失われるため、必ず終了確認に倒す（NFR-37）。
func DecideClose(quitting, trayAvailable bool) CloseAction {
	switch {
	case quitting:
		return CloseAllow
	case trayAvailable:
		return CloseHide
	default:
		return CloseConfirmQuit
	}
}

// CloseHost は CloseController が閉じる操作の結果を実行するためのウィンドウ側の操作。
// Wails の型を持たず、Shell（app.go）が runtime.* で実装する。
type CloseHost interface {
	// HideWindow はウィンドウを非表示にする。
	HideWindow()
	// ConfirmQuit は終了確認ダイアログを表示し、承認されたら true を返す。
	ConfirmQuit() bool
	// RequestQuit は確認なしの終了を要求する（Lifecycle.RequestQuit(false)）。
	RequestQuit()
}

// CloseController は OnBeforeClose の分岐を担う。
type CloseController struct {
	host          CloseHost
	quitting      func() bool
	trayAvailable func() bool
}

// NewCloseController は CloseController を返す。
// quitting は Lifecycle.Quitting、trayAvailable は起動時に 1 回判定したトレイ可否を返す。
func NewCloseController(host CloseHost, quitting, trayAvailable func() bool) *CloseController {
	return &CloseController{host: host, quitting: quitting, trayAvailable: trayAvailable}
}

// BeforeClose は OnBeforeClose の本体。prevent を返す。
func (c *CloseController) BeforeClose() (prevent bool) {
	action := DecideClose(c.quitting(), c.trayAvailable())
	switch action {
	case CloseHide:
		c.host.HideWindow()
	case CloseConfirmQuit:
		if c.host.ConfirmQuit() {
			c.host.RequestQuit()
		}
	case CloseAllow:
	}
	return action.Prevent()
}
