package organisms

import (
	"fmt"
	"testing"

	"github.com/ousiassllc/moleport/internal/tui"
)

func TestNewLogPanel(t *testing.T) {
	p := NewLogPanel()
	if p.OutputLen() != 0 {
		t.Errorf("NewLogPanel OutputLen = %d, want 0", p.OutputLen())
	}
}

func TestLogPanel_AppendOutput_And_OutputLen(t *testing.T) {
	p := NewLogPanel()
	p.AppendOutput("line1", tui.LogInfo)
	if p.OutputLen() != 1 {
		t.Errorf("OutputLen after 1 append = %d, want 1", p.OutputLen())
	}
	p.AppendOutput("line2\nline3", tui.LogInfo)
	if p.OutputLen() != 3 {
		t.Errorf("OutputLen after multiline append = %d, want 3", p.OutputLen())
	}
}

func TestLogPanel_AppendOutput_TruncatesAt100(t *testing.T) {
	p := NewLogPanel()
	for i := range 120 {
		p.AppendOutput(fmt.Sprintf("line-%d", i), tui.LogInfo)
	}
	if p.OutputLen() != 100 {
		t.Errorf("OutputLen after 120 appends = %d, want 100", p.OutputLen())
	}
	// Oldest lines should be trimmed; last line should be "line-119"
	if p.output[99].text != "line-119" {
		t.Errorf("last line = %q, want %q", p.output[99].text, "line-119")
	}
	if p.output[0].text != "line-20" {
		t.Errorf("first line = %q, want %q", p.output[0].text, "line-20")
	}
}

func TestLogPanel_SetSize(t *testing.T) {
	p := NewLogPanel()
	p.SetSize(80, 24)
	if p.width != 80 || p.height != 24 {
		t.Errorf("SetSize: got %dx%d, want 80x24", p.width, p.height)
	}
}

func TestLogPanel_View(t *testing.T) {
	p := NewLogPanel()
	p.SetSize(40, 10)
	if p.View() == "" {
		t.Error("empty View() should return non-empty string")
	}
	p.AppendOutput("hello world", tui.LogInfo)
	if p.View() == "" {
		t.Error("View() with output should return non-empty string")
	}
}

func TestLogPanel_AppendOutput_PreservesLevel(t *testing.T) {
	p := NewLogPanel()
	p.AppendOutput("error msg", tui.LogError)
	p.AppendOutput("success msg", tui.LogSuccess)
	p.AppendOutput("info msg", tui.LogInfo)
	if p.output[0].level != tui.LogError {
		t.Errorf("entry[0].level = %d, want LogError", p.output[0].level)
	}
	if p.output[1].level != tui.LogSuccess {
		t.Errorf("entry[1].level = %d, want LogSuccess", p.output[1].level)
	}
	if p.output[2].level != tui.LogInfo {
		t.Errorf("entry[2].level = %d, want LogInfo", p.output[2].level)
	}
}

func TestLogPanel_MultilinePreservesLevel(t *testing.T) {
	p := NewLogPanel()
	p.AppendOutput("line1\nline2", tui.LogError)
	if p.OutputLen() != 2 {
		t.Fatalf("OutputLen = %d, want 2", p.OutputLen())
	}
	for i, entry := range p.output {
		if entry.level != tui.LogError {
			t.Errorf("entry[%d].level = %d, want LogError", i, entry.level)
		}
	}
}

func TestStyleLogEntry(t *testing.T) {
	tests := []struct {
		name  string
		entry logEntry
		want  string
	}{
		{"empty", logEntry{}, ""},
		{"info", logEntry{text: "hello", level: tui.LogInfo}, "not_empty"},
		{"error", logEntry{text: "fail", level: tui.LogError}, "not_empty"},
		{"success", logEntry{text: "done", level: tui.LogSuccess}, "not_empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := styleLogEntry(tt.entry)
			if tt.want == "" && got != "" {
				t.Errorf("expected empty, got %q", got)
			}
			if tt.want == "not_empty" && got == "" {
				t.Errorf("expected non-empty for %v", tt.entry)
			}
		})
	}
}
