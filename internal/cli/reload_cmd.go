package cli

import (
	"fmt"

	"github.com/ousiassllc/moleport/internal/ipc"
)

// RunReload は reload サブコマンドを実行する。
func RunReload(configDir string, args []string) {
	client := connectDaemon(configDir)
	defer client.Close()

	ctx, cancel := callCtx()
	defer cancel()

	var result ipc.HostReloadResult
	if err := client.Call(ctx, "host.reload", nil, &result); err != nil {
		exitError("SSH config の再読み込みに失敗しました: %v", err)
	}

	fmt.Println("SSH config を再読み込みしました")
	fmt.Printf("  %d ホスト読み込み（新規: %d, 削除: %d）\n",
		result.Total, len(result.Added), len(result.Removed))

	for _, name := range result.Added {
		fmt.Printf("  + %s が追加されました\n", name)
	}
	for _, name := range result.Removed {
		fmt.Printf("  - %s が削除されました\n", name)
	}
}
