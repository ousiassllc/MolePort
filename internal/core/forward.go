package core

// ForwardManager はポートフォワーディングルールとセッションを管理する。
type ForwardManager interface {
	// AddRule はフォワーディングルールを追加し、割り当てられたルール名を返す。
	// Name が空の場合は自動生成される。同名ルールが存在する場合はエラーを返す。
	AddRule(rule ForwardRule) (string, error)

	// DeleteRule は指定名のルールを削除する。アクティブなセッションがあれば先に停止する。
	DeleteRule(name string) error

	// GetRules は登録済みの全ルールを追加順に返す。
	GetRules() []ForwardRule

	// GetRulesByHost は指定ホストに紐づくルールのみを追加順に返す。
	GetRulesByHost(hostName string) []ForwardRule

	// StartForward は指定ルールのポートフォワーディングを開始する。
	// 必要に応じて SSH 接続を確立し、リスナーを作成して accept ループを起動する。
	// cb が非 nil の場合、SSH 接続にクレデンシャルコールバックを使用する。
	StartForward(ruleName string, cb CredentialCallback) error

	// StopForward は指定ルールのフォワーディングセッションを停止する。
	// アクティブでない場合はエラーなしで何もしない。
	StopForward(ruleName string) error

	// StopAllForwards は全てのアクティブなフォワーディングセッションを停止する。
	StopAllForwards() error

	// GetSession は指定ルールの現在のセッション情報を返す。
	// アクティブでないルールには Status=Stopped のセッションを返す。
	GetSession(ruleName string) (*ForwardSession, error)

	// GetAllSessions は全ルールのセッション情報を追加順に返す。
	GetAllSessions() []ForwardSession

	// MarkReconnecting は当該ホストのアクティブセッションを SessionReconnecting 状態にする。
	MarkReconnecting(hostName string)

	// RestoreForwards は SSH 再接続後に SessionReconnecting 状態の全フォワードを復元する。
	RestoreForwards(hostName string) []ForwardRestoreResult

	// FailReconnecting は再接続失敗時に SessionReconnecting 状態のフォワードを Error 状態にする。
	FailReconnecting(hostName string)

	// Subscribe はフォワーディングイベントを受信するチャネルを返す。
	Subscribe() <-chan ForwardEvent

	// Close は全フォワーディングを停止し、サブスクライバーチャネルを閉じる。
	Close()
}
