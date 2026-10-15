package service

// Topic は WebView に「再取得すべき領域」を伝えるトピック名。
// Wails Events のペイロードにはトピック名のみを載せる（データ・クレデンシャルは含めない）。
type Topic string

const (
	TopicHosts      Topic = "hosts"
	TopicForwards   Topic = "forwards"
	TopicStatus     Topic = "status"
	TopicHeader     Topic = "header"
	TopicCredential Topic = "credential"
	TopicToast      Topic = "toast"
	TopicSettings   Topic = "settings"
	TopicAll        Topic = "all"
)

// Notifier は更新トピックを WebView へ通知する。
// Shell が Wails Events（runtime.EventsEmit）で実装して注入するため、
// Web / Service 層は Wails の型に依存しない。
type Notifier interface {
	Notify(topics ...Topic)
}
