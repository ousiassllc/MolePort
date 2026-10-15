package service

// LifecycleState は内蔵デーモンの起動状態。
type LifecycleState string

const (
	// LifecycleStarting は内蔵デーモンの起動処理中（引き継ぎ・Start 待ち）。
	LifecycleStarting LifecycleState = "starting"
	// LifecycleReady は内蔵デーモンが起動完了し、操作を受け付けられる状態。
	LifecycleReady LifecycleState = "ready"
	// LifecycleFailed は内蔵デーモンの起動に失敗した状態。
	LifecycleFailed LifecycleState = "failed"
)

// LifecycleSource は現在の起動状態を返す。Shell の Lifecycle が実装する。
type LifecycleSource interface {
	State() LifecycleState
}
