package shell

import (
	"sync/atomic"

	"github.com/ousiassllc/moleport/internal/gui/service"
)

// ChangedEvent は WebView へ更新トピックを伝える Wails Events のイベント名。
// ペイロードは {"topics": ["hosts", ...]} でトピック名のみを含む（NFR-36）。
const ChangedEvent = "moleport:changed"

// VisibilityNotifier は service.Notifier の実装。ウィンドウ表示中だけ emit を呼び（NFR-33）、
// 非表示から表示へ戻った時に全トピック（all）を 1 回送って再同期する。
// emit は Wails の runtime.EventsEmit を包んだ関数で、Wails 型はここに現れない。
type VisibilityNotifier struct {
	visible atomic.Bool
	emit    func(topics []string)
}

var _ service.Notifier = (*VisibilityNotifier)(nil)

// NewVisibilityNotifier は初期状態を非表示（DOM 読み込み前）として生成する。
func NewVisibilityNotifier(emit func(topics []string)) *VisibilityNotifier {
	return &VisibilityNotifier{emit: emit}
}

// Notify はウィンドウ表示中のみトピックを送る。非表示中の通知は破棄する。
func (n *VisibilityNotifier) Notify(topics ...service.Topic) {
	if len(topics) == 0 || !n.visible.Load() {
		return
	}
	n.emit(topicNames(topics))
}

// SetVisible は表示状態を更新する。非表示 → 表示の遷移時に all を 1 回送る。
func (n *VisibilityNotifier) SetVisible(visible bool) {
	if was := n.visible.Swap(visible); visible && !was {
		n.emit(topicNames([]service.Topic{service.TopicAll}))
	}
}

// Visible は現在の表示状態を返す。
func (n *VisibilityNotifier) Visible() bool { return n.visible.Load() }

func topicNames(topics []service.Topic) []string {
	names := make([]string, len(topics))
	for i, t := range topics {
		names[i] = string(t)
	}
	return names
}
