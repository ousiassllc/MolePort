package tray

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// MaxForwardItems はトレイメニューに並べる転送項目の上限（項目プールの大きさ）。超過分は More 項目に集約する。
const MaxForwardItems = 20

// ForwardItem は転送ルール 1 件のチェック項目。Checked は稼働中・開始中・再接続中（クリックで停止）。
type ForwardItem struct {
	Name    string // 転送ルール名（クリック時に ToggleForward へ渡す）
	Title   string
	Checked bool
}

// Menu はトレイメニュー全体の内容。ラベルは現在の言語で解決済みで、項目の追加・削除なしに
// 既存項目の SetTitle / Check / Show / Hide だけで反映できる（UC-33）。
type Menu struct {
	Summary        string // 状態サマリー（無効項目）
	Show           string
	Forwards       []ForwardItem // 最大 MaxForwardItems 件
	More           string        // 転送が MaxForwardItems 件を超える時だけ非空（「ウィンドウで全て表示…」）
	StopAll        string
	StopAllEnabled bool
	Quit           string
}

// BuildMenu は ViewModel からメニューの項目モデルを組み立てる純粋関数（Wails・systray に依存しない）。
func BuildMenu(status service.StatusView, forwards []service.ForwardView) Menu {
	m := Menu{
		Summary: i18n.T("gui.tray.summary", map[string]any{"Active": status.ActiveForwards, "Hosts": status.ConnectedHosts}),
		Show:    i18n.T("gui.tray.show"),
		StopAll: i18n.T("gui.tray.stop_all"),
		Quit:    i18n.T("gui.tray.quit"),
	}
	for _, f := range forwards {
		if f.Actions.CanStop {
			m.StopAllEnabled = true
		}
		if len(m.Forwards) < MaxForwardItems {
			m.Forwards = append(m.Forwards, ForwardItem{
				Name:    f.Name,
				Title:   fmt.Sprintf("%s (%s)", f.Name, f.Route),
				Checked: f.Actions.CanStop,
			})
		}
	}
	if len(forwards) > MaxForwardItems {
		m.More = i18n.T("gui.tray.more")
	}
	return m
}

// Source はメニューの元になる ViewModel の取得元。service.Dashboard の一部。
type Source interface {
	Status() service.StatusView
	Forwards() []service.ForwardView
}

// BuildFrom は src の現在の状態からメニューを組み立てる。
func BuildFrom(src Source) Menu {
	return BuildMenu(src.Status(), src.Forwards())
}
