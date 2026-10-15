package daemon

import "os"

// IsDaemonMode は os.Args に --daemon-mode フラグが含まれているかを返す。
func IsDaemonMode() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--daemon-mode" {
			return true
		}
	}
	return false
}
