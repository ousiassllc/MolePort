package infra

import "os"

// homeDir はカレントユーザーのホームディレクトリを返す。
// os.UserHomeDir が失敗した場合は HOME 環境変数にフォールバックする。
func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.Getenv("HOME")
	}
	return home
}
