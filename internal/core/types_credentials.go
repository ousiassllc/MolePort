package core

// CredentialType はクレデンシャル要求の種別を表す。
type CredentialType string

const (
	CredentialPassword            CredentialType = "password"
	CredentialPassphrase          CredentialType = "passphrase"
	CredentialKeyboardInteractive CredentialType = "keyboard-interactive"
)

// PromptInfo は keyboard-interactive 認証の個別プロンプト情報。
type PromptInfo struct {
	Prompt string
	Echo   bool
}

// CredentialRequest はクレデンシャル要求を表す。
type CredentialRequest struct {
	RequestID string
	Type      CredentialType
	Host      string
	Prompt    string       // password/passphrase 用
	Prompts   []PromptInfo // keyboard-interactive 用
}

// CredentialResponse はクレデンシャル応答を表す。
type CredentialResponse struct {
	RequestID string
	Value     string   // password/passphrase 用
	Answers   []string // keyboard-interactive 用
	Cancelled bool
}

// CredentialCallback はクレデンシャル要求時に呼び出されるコールバック関数の型。
// デーモンがクライアントにクレデンシャルを要求し、応答を受け取る。
type CredentialCallback func(req CredentialRequest) (CredentialResponse, error)
