// Package static は esbuild / Tailwind が生成した静的アセット（dist/）を go:embed で保持する。
// dist/ は git 管理せず（.gitkeep のみ）、make gui-assets で生成する。
package static

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS は dist/ を含むファイルシステムを返す。
// ルーターの /static/{path...} に対し、path は "dist/js/app.js" のように解決される。
func FS() fs.FS { return dist }
