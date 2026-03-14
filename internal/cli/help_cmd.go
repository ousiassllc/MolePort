package cli

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/i18n"
)

// RunHelp は help サブコマンドを実行する。
func RunHelp(configDir string, args []string) {
	fmt.Print(i18n.T("cli.help.text"))
}
