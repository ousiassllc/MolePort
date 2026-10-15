package service

// ViewModel は Core の型から組み立てる表示用の構造体。永続化しない。
// 整形済み文字列とアクション可否を持ち、Templ 側に条件判断を持ち込まない。

// HostActions はホスト行で実行可能な操作（HATEOAS: true のものだけを描画する）。
type HostActions struct {
	CanConnect    bool // Disconnected / Error / PendingAuth
	CanDisconnect bool // Connected / Reconnecting
	CanAddForward bool // 常に true
}

// ForwardActions は転送行で実行可能な操作。
type ForwardActions struct {
	CanStart  bool // Stopped / Error
	CanStop   bool // Active / Reconnecting（Starting を含む）
	CanDelete bool // 常に true（確認ダイアログ経由）
}

// HostView はホスト行の表示内容。
type HostView struct {
	Name        string
	Address     string // "user@host:port"
	State       string // "connected" | "disconnected" | "reconnecting" | "pending_auth" | "error"
	StateLabel  string // ローカライズ済み
	ActiveCount int
	Actions     HostActions
}

// ForwardView は転送行の表示内容。
type ForwardView struct {
	Name        string
	Host        string
	Type        string // "local" | "remote" | "dynamic"
	Route       string // ":8080 → localhost:80" 等
	Status      string // "active" | "stopped" | "error" | "reconnecting"
	StatusLabel string // ローカライズ済み
	Uptime      string // "2h 15m"（停止中は空）
	Sent        string // "1.2 MB"
	Received    string
	Reconnects  int
	LastError   string
	Actions     ForwardActions
}

// StatusView はステータスバーの表示内容。
type StatusView struct {
	Hosts, ConnectedHosts, Forwards, ActiveForwards int
}

// HeaderView はヘッダーの表示内容。
type HeaderView struct {
	Version         string
	PID             int
	UpdateAvailable bool
	LatestVersion   string
	ReleaseURL      string
}

// FormErrors はフィールド名 → ローカライズ済みエラーメッセージ（"_form" は全体エラー）。
type FormErrors map[string]string

// ToastKind はトーストの種別。
type ToastKind string

const (
	ToastInfo    ToastKind = "info"
	ToastSuccess ToastKind = "success"
	ToastError   ToastKind = "error"
)

// ToastView は非同期通知（トースト）の表示内容。
type ToastView struct {
	Kind    ToastKind
	Message string // ローカライズ済み。クレデンシャル・内部パスを含めない
}
