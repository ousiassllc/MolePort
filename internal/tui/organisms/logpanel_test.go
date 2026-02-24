package organisms

import (
	"strings"
	"testing"
)

func TestStyleLogLine_FailureWithShimashita(t *testing.T) {
	// "しました" を含むが全体としては失敗メッセージのケース。
	// "失敗" が含まれるので ✗ が表示されるべき。
	line := "ルール 'test-rule' を追加しましたが、開始に失敗: rpc error"
	got := styleLogLine(line)
	if strings.Contains(got, "✓") {
		t.Errorf("styleLogLine(%q) should not contain ✓ for failure message, got %q", line, got)
	}
	if !strings.Contains(got, "✗") {
		t.Errorf("styleLogLine(%q) should contain ✗ for failure message, got %q", line, got)
	}
}

func TestStyleLogLine_PureSuccess(t *testing.T) {
	line := "ルール 'test-rule' を追加し、開始しました"
	got := styleLogLine(line)
	if !strings.Contains(got, "✓") {
		t.Errorf("styleLogLine(%q) should contain ✓ for success message, got %q", line, got)
	}
	if strings.Contains(got, "✗") {
		t.Errorf("styleLogLine(%q) should not contain ✗ for success message, got %q", line, got)
	}
}

func TestStyleLogLine_ErrorKeyword(t *testing.T) {
	line := "ルール追加エラー: something went wrong"
	got := styleLogLine(line)
	if !strings.Contains(got, "✗") {
		t.Errorf("styleLogLine(%q) should contain ✗, got %q", line, got)
	}
}

func TestStyleLogLine_EmptyLine(t *testing.T) {
	got := styleLogLine("")
	if got != "" {
		t.Errorf("styleLogLine(\"\") = %q, want empty string", got)
	}
}
