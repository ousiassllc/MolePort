package shell

import "strings"

// IsAffirmative は QuestionDialog の戻り値が「はい」かを判定する。
// Wails の MessageDialog は OS により戻り値が異なる（macOS は押下したボタンのラベル、
// Windows は "Yes" / "No" 固定）ため、ローカライズ済みラベルと英語の "Yes" の両方を肯定として扱う。
// 実機での戻り値は未確認（docs/architecture/gui.md R-5 の確認項目）。
func IsAffirmative(result, yesLabel string) bool {
	result = strings.TrimSpace(result)
	return strings.EqualFold(result, yesLabel) || strings.EqualFold(result, "yes")
}
