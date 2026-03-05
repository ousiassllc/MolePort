package theme

import (
	"log"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// Palette はテーマのカラーパレットを定義する。
type Palette struct {
	Accent      lipgloss.Color
	AccentDim   lipgloss.Color
	Text        lipgloss.Color
	Muted       lipgloss.Color
	Dim         lipgloss.Color
	Error       lipgloss.Color
	Warning     lipgloss.Color
	BgHighlight lipgloss.Color
}

// Preset はテーマプリセットを定義する。
type Preset struct {
	ID      string
	Base    string // "dark" | "light"
	Accent  string // "violet" | "blue" | "green" | "cyan" | "orange"
	Label   string // 表示名
	Palette Palette
}

var (
	current Palette
	mu      sync.RWMutex // current のみを保護する。presets, presetOrder は初期化後に不変。
)

func init() {
	p, ok := presets[DefaultPresetID()]
	if !ok {
		log.Fatal("default preset not found: " + DefaultPresetID())
	}
	current = p.Palette
}

// Current は現在適用されている Palette を返す。
func Current() Palette {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Apply は指定された presetID のパレットを適用する。
// 存在しない ID の場合は何もしない。
func Apply(presetID string) {
	p, ok := presets[presetID]
	if !ok {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	current = p.Palette
}

// Presets は全プリセットを定義順で返す。
func Presets() []Preset {
	result := make([]Preset, len(presetOrder))
	for i, id := range presetOrder {
		result[i] = presets[id]
	}
	return result
}

// PresetsByBase は指定された base ("dark" or "light") のプリセットを定義順で返す。
func PresetsByBase(base string) []Preset {
	var result []Preset
	for _, id := range presetOrder {
		if presets[id].Base == base {
			result = append(result, presets[id])
		}
	}
	return result
}

// FindPreset は指定された ID のプリセットを返す。
func FindPreset(id string) (Preset, bool) {
	p, ok := presets[id]
	return p, ok
}

// DefaultPresetID はデフォルトのプリセット ID を返す。
func DefaultPresetID() string {
	return "dark-violet"
}

// PresetIDFromConfig は base と accent からプリセット ID を生成する。
func PresetIDFromConfig(base, accent string) string {
	return base + "-" + accent
}
