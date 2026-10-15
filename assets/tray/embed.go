// Package trayassets はシステムトレイのアイコンを go:embed で保持する。
// 現在のアイコンはプレースホルダー（単色の円）。製品版のアイコンに差し替える際はファイルのみ置き換える。
package trayassets

import _ "embed"

// MacTemplate は macOS のテンプレート画像（ダーク/ライト自動反転。黒 + 透過）。
//
//go:embed mac-template.png
var MacTemplate []byte

// PNG は Linux 用のトレイアイコン（macOS では SetTemplateIcon の通常アイコンにも使う）。
//
//go:embed tray.png
var PNG []byte

// ICO は Windows 用のトレイアイコン（systray は Windows で ICO 形式を要求する）。
//
//go:embed tray.ico
var ICO []byte
