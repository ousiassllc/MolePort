//go:build !windows

package pidfile

import (
	"errors"
	"os"
	"syscall"
)

// lockFile は f に非ブロッキングで排他ロック（flock）を取得する。
// 他プロセスが保持している場合は isLockHeld が true になるエラーを返す。
func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) //nolint:gosec // Fd() は有効な fd を返す
}

// isLockHeld は lockFile のエラーが「他プロセスがロックを保持中」を表すかを返す。
func isLockHeld(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK)
}

// unlockFile は f のロックを解放する。
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:gosec // Fd() は有効な fd を返す
}

// releaseFile は PID ファイルを削除し、ロックを解放してファイルを閉じる。
// ロック解放前に削除することで、次のデーモンが削除前の inode をロックする競合を防ぐ。
func releaseFile(path string, f *os.File) error {
	removeErr := os.Remove(path)
	unlockErr := unlockFile(f)
	closeErr := f.Close()
	return errors.Join(removeErr, unlockErr, closeErr)
}

// processExists は kill(pid, 0) でプロセスの存在を確認する。
func processExists(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// killProcess は pid のプロセスを SIGKILL で強制終了する。
func killProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}
