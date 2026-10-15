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

// QuitController は CloseController が参照・要求する終了制御。*Lifecycle が満たす。
type QuitController interface {
	Quitting() bool
	ActiveForwards() int
	// RequestQuit は終了を要求する。confirm が false なら確認しない。
	RequestQuit(confirm bool)
}

// CloseController は OnBeforeClose の分岐を担う。
type CloseController struct {
	win           Window
	dialog        Dialog
	life          QuitController
	trayAvailable func() bool
	onHidden      func()
}

// NewCloseController は CloseController を返す。
// trayAvailable は起動時に 1 回判定したトレイ可否を返す。onHidden はトレイへ非表示にした直後に呼ばれる
// （初回のみのトレイ稼働通知。nil 可）。
func NewCloseController(win Window, dialog Dialog, life QuitController, trayAvailable func() bool, onHidden func()) *CloseController {
	return &CloseController{win: win, dialog: dialog, life: life, trayAvailable: trayAvailable, onHidden: onHidden}
}

// BeforeClose は OnBeforeClose の本体。prevent を返す。
func (c *CloseController) BeforeClose() (prevent bool) {
	action := DecideClose(c.life.Quitting(), c.trayAvailable())
	switch action {
	case CloseHide:
		c.win.Hide()
		if c.onHidden != nil {
			c.onHidden()
		}
	case CloseConfirmQuit:
		// ダイアログで確認済みのため RequestQuit は再確認しない。
		if c.dialog.ConfirmClose(c.life.ActiveForwards()) {
			c.life.RequestQuit(false)
		}
	case CloseAllow:
	}
	return action.Prevent()
}
