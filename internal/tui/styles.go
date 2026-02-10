package tui

import "github.com/charmbracelet/lipgloss"

// カラーパレット
var (
	ColorActive       = lipgloss.Color("#00FF00") // 緑: 接続中/アクティブ
	ColorStopped      = lipgloss.Color("#666666") // 暗灰: 停止/切断
	ColorError        = lipgloss.Color("#FF0000") // 赤: エラー
	ColorReconnecting = lipgloss.Color("#FFFF00") // 黄: 再接続中/接続中
	ColorSelected     = lipgloss.Color("#7D56F4") // 紫: 選択中
	ColorMuted        = lipgloss.Color("#888888") // 灰: 補助テキスト
	ColorText         = lipgloss.Color("#FAFAFA") // 白: 通常テキスト
	ColorBorder       = lipgloss.Color("#555555") // 灰: ボーダー
	ColorFocusBorder  = lipgloss.Color("#7D56F4") // 紫: フォーカス中のボーダー
)

// パネルスタイル
var (
	PanelBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	PanelBorderFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorFocusBorder).
				Padding(0, 1)
)

// テキストスタイル
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	SelectedStyle = lipgloss.NewStyle().
			Background(ColorSelected).
			Foreground(ColorText).
			Bold(true)
)

// ステータスカラースタイル
var (
	ActiveStyle       = lipgloss.NewStyle().Foreground(ColorActive)
	StoppedStyle      = lipgloss.NewStyle().Foreground(ColorStopped)
	ErrorStyle        = lipgloss.NewStyle().Foreground(ColorError)
	ReconnectingStyle = lipgloss.NewStyle().Foreground(ColorReconnecting)
)

// キーヒントスタイル
var (
	KeyStyle  = lipgloss.NewStyle().Foreground(ColorMuted).Bold(true)
	DescStyle = lipgloss.NewStyle().Foreground(ColorMuted)
)

// 区切り線スタイル
var DividerStyle = lipgloss.NewStyle().Foreground(ColorBorder)
