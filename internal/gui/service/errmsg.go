package service

import (
	"errors"
	"strings"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// LocalizeError は Core のエラーをローカライズ済みの短い説明に変換する。
// Core の英語エラー文字列を直接見せないため、既知のエラー型は i18n キーへ写像する。
// 未分類のエラーのみ原因の切り分けに必要な詳細（クレデンシャルを含まない Core のエラー文字列）を添える。
func LocalizeError(err error) string {
	if err == nil {
		return ""
	}

	var notFound *core.NotFoundError
	var exists *core.AlreadyExistsError
	var active *core.AlreadyActiveError
	var notConnected *core.NotConnectedError
	var authRequired *core.AuthRequiredError

	switch {
	case errors.Is(err, core.ErrCredentialTimeout):
		return i18n.T("gui.error.credential_timeout")
	case errors.Is(err, core.ErrCredentialCancelled):
		return i18n.T("gui.error.credential_cancelled")
	case errors.As(err, &notFound):
		return i18n.T("gui.error.not_found", map[string]any{"Name": notFound.Name})
	case errors.As(err, &exists):
		return i18n.T("gui.error.already_exists", map[string]any{"Name": exists.Name})
	case errors.As(err, &active):
		return i18n.T("gui.error.already_active", map[string]any{"Name": active.Name})
	case errors.As(err, &notConnected):
		return i18n.T("gui.error.not_connected", map[string]any{"Name": notConnected.HostName})
	case errors.As(err, &authRequired), core.IsAuthFailure(err):
		return i18n.T("gui.error.auth_failed")
	case strings.Contains(err.Error(), "address already in use"):
		return i18n.T("gui.error.port_in_use")
	default:
		return i18n.T("gui.error.unknown", map[string]any{"Error": err.Error()})
	}
}
