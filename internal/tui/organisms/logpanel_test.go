package organisms

import (
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/tui"
)

func TestStyleLogEntry_Error(t *testing.T) {
	got := styleLogEntry(logEntry{text: "ルール追加エラー: something went wrong", level: tui.LogError})
	if !strings.Contains(got, "✗") {
		t.Errorf("styleLogEntry(LogError) should contain ✗, got %q", got)
	}
	if strings.Contains(got, "✓") {
		t.Errorf("styleLogEntry(LogError) should not contain ✓, got %q", got)
	}
}

func TestStyleLogEntry_Success(t *testing.T) {
	got := styleLogEntry(logEntry{text: "ルール 'test-rule' を追加し、開始しました", level: tui.LogSuccess})
	if !strings.Contains(got, "✓") {
		t.Errorf("styleLogEntry(LogSuccess) should contain ✓, got %q", got)
	}
	if strings.Contains(got, "✗") {
		t.Errorf("styleLogEntry(LogSuccess) should not contain ✗, got %q", got)
	}
}

func TestStyleLogEntry_Info(t *testing.T) {
	got := styleLogEntry(logEntry{text: "some info message", level: tui.LogInfo})
	if strings.Contains(got, "✗") || strings.Contains(got, "✓") {
		t.Errorf("styleLogEntry(LogInfo) should not contain ✗ or ✓, got %q", got)
	}
	if got == "" {
		t.Error("styleLogEntry(LogInfo) should return non-empty for non-empty text")
	}
}

func TestStyleLogEntry_EmptyLine(t *testing.T) {
	got := styleLogEntry(logEntry{})
	if got != "" {
		t.Errorf("styleLogEntry(empty) = %q, want empty string", got)
	}
}

func TestStyleLogEntry_ErrorTakesPrecedenceOverContent(t *testing.T) {
	// テキストに "started" が含まれていても、Level が LogError なら ✗ が表示される
	got := styleLogEntry(logEntry{text: "Rule 'test-rule' started but failed", level: tui.LogError})
	if !strings.Contains(got, "✗") {
		t.Errorf("styleLogEntry(LogError) should contain ✗ regardless of text content, got %q", got)
	}
	if strings.Contains(got, "✓") {
		t.Errorf("styleLogEntry(LogError) should not contain ✓, got %q", got)
	}
}

func TestStyleLogEntry_SuccessIgnoresErrorKeywords(t *testing.T) {
	// テキストに "error" が含まれていても、Level が LogSuccess なら ✓ が表示される
	got := styleLogEntry(logEntry{text: "recovered from error successfully", level: tui.LogSuccess})
	if !strings.Contains(got, "✓") {
		t.Errorf("styleLogEntry(LogSuccess) should contain ✓ regardless of text content, got %q", got)
	}
	if strings.Contains(got, "✗") {
		t.Errorf("styleLogEntry(LogSuccess) should not contain ✗, got %q", got)
	}
}
