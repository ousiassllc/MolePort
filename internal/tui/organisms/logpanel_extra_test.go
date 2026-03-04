package organisms

import (
	"fmt"
	"testing"
)

func TestNewLogPanel(t *testing.T) {
	p := NewLogPanel()
	if p.OutputLen() != 0 {
		t.Errorf("NewLogPanel OutputLen = %d, want 0", p.OutputLen())
	}
}

func TestLogPanel_AppendOutput_And_OutputLen(t *testing.T) {
	p := NewLogPanel()
	p.AppendOutput("line1")
	if p.OutputLen() != 1 {
		t.Errorf("OutputLen after 1 append = %d, want 1", p.OutputLen())
	}
	p.AppendOutput("line2\nline3")
	if p.OutputLen() != 3 {
		t.Errorf("OutputLen after multiline append = %d, want 3", p.OutputLen())
	}
}

func TestLogPanel_AppendOutput_TruncatesAt100(t *testing.T) {
	p := NewLogPanel()
	for i := range 120 {
		p.AppendOutput(fmt.Sprintf("line-%d", i))
	}
	if p.OutputLen() != 100 {
		t.Errorf("OutputLen after 120 appends = %d, want 100", p.OutputLen())
	}
	// Oldest lines should be trimmed; last line should be "line-119"
	if p.output[99] != "line-119" {
		t.Errorf("last line = %q, want %q", p.output[99], "line-119")
	}
	if p.output[0] != "line-20" {
		t.Errorf("first line = %q, want %q", p.output[0], "line-20")
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
	p.AppendOutput("hello world")
	if p.View() == "" {
		t.Error("View() with output should return non-empty string")
	}
}
