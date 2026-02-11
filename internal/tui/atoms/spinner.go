package atoms

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/ousiassllc/moleport/internal/tui"
)

// NewSpinner は処理中を示すスピナーモデルを返す。
func NewSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = tui.ReconnectingStyle
	return s
}
