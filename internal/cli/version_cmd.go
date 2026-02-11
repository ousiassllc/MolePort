package cli

import (
	"fmt"
	"runtime"
)

// Version はビルド時に -ldflags で設定されるバージョン情報。
var Version = "dev"

// RunVersion は version サブコマンドを実行する。
func RunVersion(configDir string, args []string) {
	fmt.Printf("MolePort %s (%s, %s/%s)\n", Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
