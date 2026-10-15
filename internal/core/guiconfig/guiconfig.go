// Package guiconfig は config.yaml の gui: セクション（moleport-gui のみ参照。TUI の設定とは独立）。
// core.Config.GUI の型・既定値・検証を持つ（core のディレクトリ行数制限のためサブパッケージに置く）。
package guiconfig

import "slices"

// テーマの既定値。
const (
	DefaultBase   = "system"
	DefaultAccent = "violet"
)

// bases は gui.theme.base に指定できる値。system は OS の prefers-color-scheme に追従する。
var bases = []string{"system", "dark", "light"}

// accents は gui.theme.accent に指定できる値。
var accents = []string{"violet", "blue", "green", "cyan", "orange"}

// Config は config.yaml の gui: セクション。
type Config struct {
	Theme           Theme `yaml:"theme"`
	TrayNoticeShown bool  `yaml:"tray_notice_shown"` // 「トレイで稼働しています」通知を表示済みか
}

// Theme は GUI のテーマ設定。
type Theme struct {
	Base   string `yaml:"base"`   // "system" | "dark" | "light"
	Accent string `yaml:"accent"` // "violet" | "blue" | "green" | "cyan" | "orange"
}

// Default は gui: セクションの既定値を返す。core.DefaultConfig が使うため、
// gui: のない既存の config.yaml はこの値で読み込まれる。
func Default() Config {
	return Config{Theme: Theme{Base: DefaultBase, Accent: DefaultAccent}}
}

// Bases は gui.theme.base に指定できる値を返す。
func Bases() []string { return slices.Clone(bases) }

// Accents は gui.theme.accent に指定できる値を返す。
func Accents() []string { return slices.Clone(accents) }

// ValidBase は base が gui.theme.base に指定できる値かを返す。
func ValidBase(base string) bool { return slices.Contains(bases, base) }

// ValidAccent は accent が gui.theme.accent に指定できる値かを返す。
func ValidAccent(accent string) bool { return slices.Contains(accents, accent) }

// Normalized は空・不正な値を既定値に置き換えたテーマを返す（手で編集された config.yaml への備え）。
func (t Theme) Normalized() Theme {
	if !ValidBase(t.Base) {
		t.Base = DefaultBase
	}
	if !ValidAccent(t.Accent) {
		t.Accent = DefaultAccent
	}
	return t
}
