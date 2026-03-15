package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// センチネルエラー
var (
	ErrCredentialTimeout   = errors.New("credential timeout")
	ErrCredentialCancelled = errors.New("credential cancelled")
)

// NotFoundError はリソースが見つからないエラー。
type NotFoundError struct {
	Resource string // "host" or "rule"
	Name     string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.Resource, e.Name)
}

// AlreadyExistsError はリソースが既に存在するエラー。
type AlreadyExistsError struct {
	Resource string
	Name     string
}

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("%s %q already exists", e.Resource, e.Name)
}

// AlreadyActiveError は既にアクティブなエラー。
type AlreadyActiveError struct {
	Name string
}

func (e *AlreadyActiveError) Error() string {
	return fmt.Sprintf("%q is already active", e.Name)
}

// NotConnectedError はホスト未接続エラー。
type NotConnectedError struct {
	HostName string
}

func (e *NotConnectedError) Error() string {
	return fmt.Sprintf("host %q is not connected", e.HostName)
}

// AuthRequiredError は認証が必要なエラー。
type AuthRequiredError struct {
	HostName string
	Err      error
}

func (e *AuthRequiredError) Error() string {
	return fmt.Sprintf("authentication required for %s: %v", e.HostName, e.Err)
}

func (e *AuthRequiredError) Unwrap() error {
	return e.Err
}

// authFailureMessages は認証失敗を示すエラー文字列のリスト。
var authFailureMessages = []string{
	"unable to authenticate",
	"no authentication methods available",
	"no supported methods remain",
}

// IsAuthFailure はエラーが認証失敗を示すかどうかを判定する。
func IsAuthFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, s := range authFailureMessages {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// LocalhostAddr はローカルホストの IPv4 アドレス。
const LocalhostAddr = "127.0.0.1"

// CredentialTimeout はクレデンシャル応答を待つサーバー側タイムアウト。
// TUI 側はこの値に IPC オーバーヘッド分のバッファを加算して使用する。
const CredentialTimeout = 30 * time.Second
