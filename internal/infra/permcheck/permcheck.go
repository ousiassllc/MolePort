package permcheck

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
)

// otherAccessMask は group / other のアクセス権ビット。
const otherAccessMask os.FileMode = 0o077

// Warnings は設定ディレクトリ・設定ファイル・状態ファイル・SSH config のパーミッションを検査し、
// group / other にアクセス権があるものについて警告メッセージを返す（NFR-09 / NFR-10）。
// 検査のみで権限は変更しない。存在しないパスは対象外。
// Windows はパーミッションを ACL に委ねるため検査しない（NFR-39）。
func Warnings(configDir, sshConfigPath string) []string {
	if runtime.GOOS == "windows" {
		return nil
	}

	targets := []struct {
		path, kind, want string
	}{
		{configDir, "config directory", "0700"},
		{filepath.Join(configDir, "config.yaml"), "config file", "0600"},
		{filepath.Join(configDir, "state.yaml"), "state file", "0600"},
		{sshConfigPath, "SSH config", "0600"},
	}

	var warnings []string
	for _, t := range targets {
		info, err := os.Stat(t.path)
		if err != nil {
			continue
		}
		if perm := info.Mode().Perm(); perm&otherAccessMask != 0 {
			msg := fmt.Sprintf("%s %s has permissions %04o that allow group/other access; recommended %s", t.kind, t.path, perm, t.want)
			slog.Warn("insecure file permissions", "path", t.path, "kind", t.kind, "permissions", fmt.Sprintf("%04o", perm), "recommended", t.want)
			warnings = append(warnings, msg)
		}
	}
	return warnings
}
