package protocol

// --- イベントサブスクリプション ---

// EventsSubscribeParams は events.subscribe リクエストのパラメータ。
type EventsSubscribeParams struct {
	Types []string `json:"types"`
}

// EventsSubscribeResult は events.subscribe リクエストの結果。
type EventsSubscribeResult struct {
	SubscriptionID string `json:"subscription_id"`
}

// EventsUnsubscribeParams は events.unsubscribe リクエストのパラメータ。
type EventsUnsubscribeParams struct {
	SubscriptionID string `json:"subscription_id"`
}

// EventsUnsubscribeResult は events.unsubscribe リクエストの結果。
type EventsUnsubscribeResult struct {
	OK bool `json:"ok"`
}

// --- イベント通知 ---

// SSHEventNotification は SSH イベント通知を表す。
type SSHEventNotification struct {
	Type  string `json:"type"`
	Host  string `json:"host"`
	Error string `json:"error,omitempty"`
}

// ForwardEventNotification はポートフォワーディングイベント通知を表す。
type ForwardEventNotification struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Host  string `json:"host"`
	Error string `json:"error,omitempty"`
}

// MetricsEventNotification はメトリクスイベント通知を表す。
type MetricsEventNotification struct {
	Sessions []SessionMetrics `json:"sessions"`
}

// SessionMetrics はセッションのメトリクス情報を表す。
type SessionMetrics struct {
	Name          string `json:"name"`
	Status        string `json:"status"`
	BytesSent     int64  `json:"bytes_sent"`
	BytesReceived int64  `json:"bytes_received"`
	Uptime        string `json:"uptime"`
}

// --- クレデンシャル認証 ---

// CredentialRequestNotification はデーモンからクライアントへのクレデンシャル要求通知。
type CredentialRequestNotification struct {
	RequestID string       `json:"request_id"`
	Type      string       `json:"type"` // "password" | "passphrase" | "keyboard-interactive"
	Host      string       `json:"host"`
	Prompt    string       `json:"prompt,omitempty"`
	Prompts   []PromptData `json:"prompts,omitempty"`
}

// PromptData は keyboard-interactive 認証の個別プロンプト。
type PromptData struct {
	Prompt string `json:"prompt"`
	Echo   bool   `json:"echo"`
}

// CredentialResponseParams はクライアントからデーモンへのクレデンシャル応答パラメータ。
type CredentialResponseParams struct {
	RequestID string   `json:"request_id"`
	Value     string   `json:"value,omitempty"`
	Answers   []string `json:"answers,omitempty"`
	Cancelled bool     `json:"cancelled,omitempty"`
}

// CredentialResponseResult はクレデンシャル応答の結果。
type CredentialResponseResult struct {
	OK bool `json:"ok"`
}
