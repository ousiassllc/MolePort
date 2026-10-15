package service

import "sync"

// MaxNotifications は NotificationQueue が保持する通知の上限。超過時は古いものから破棄する。
const MaxNotifications = 50

// NotificationQueue は非同期通知（接続・転送開始の失敗、引き継ぎ完了など）のメモリ上 FIFO。
// ウィンドウ非表示中も溜め、/fragments/toasts が Drain で取り出して OOB トーストとして返す。
type NotificationQueue struct {
	mu    sync.Mutex
	items []ToastView
}

// NewNotificationQueue は空のキューを返す。
func NewNotificationQueue() *NotificationQueue { return &NotificationQueue{} }

// Push は通知を末尾に追加する。上限（50 件）を超えた場合は最も古い通知を破棄する。
func (q *NotificationQueue) Push(t ToastView) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) >= MaxNotifications {
		q.items = append(q.items[:0], q.items[len(q.items)-MaxNotifications+1:]...)
	}
	q.items = append(q.items, t)
}

// Drain は溜まっている通知を古い順に全て取り出し、キューを空にする。空なら nil を返す。
func (q *NotificationQueue) Drain() []ToastView {
	q.mu.Lock()
	defer q.mu.Unlock()
	items := q.items
	q.items = nil
	return items
}
