//go:build windows

package pidfile

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockOffset は排他ロックをかけるバイト範囲の開始位置。
// Windows のロックは強制ロックで、ロック範囲は他プロセスから読めなくなる。
// PID の読み取り（IsRunning / IsLocked / KillProcess）を妨げないよう、
// データが書かれる先頭領域ではなくファイル末尾より十分先の 1 バイトをロックする。
const lockOffset = 0x7FFFFFFF

// lockFile は f に非ブロッキングで排他ロック（LockFileEx）を取得する。
// 他プロセスが保持している場合は isLockHeld が true になるエラーを返す。
func lockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	ol.Offset = lockOffset
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
}

// isLockHeld は lockFile のエラーが「他プロセスがロックを保持中」を表すかを返す。
func isLockHeld(err error) bool {
	return errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}

// unlockFile は f のロックを解放する。
func unlockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	ol.Offset = lockOffset
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)
}

// releaseFile はロックを解放してファイルを閉じ、PID ファイルを削除する。
// Windows では開いたままのファイルを削除できないため、Unix と異なり削除は最後に行う。
func releaseFile(path string, f *os.File) error {
	unlockErr := unlockFile(f)
	closeErr := f.Close()
	removeErr := os.Remove(path)
	return errors.Join(unlockErr, closeErr, removeErr)
}

// processExists は OpenProcess + GetExitCodeProcess でプロセスが実行中かを確認する。
func processExists(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid)) //nolint:gosec // pid は正の整数に検証済み
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h) //nolint:errcheck // 読み取り専用ハンドルのクローズ失敗は無視してよい

	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == 259 // STILL_ACTIVE
}

// killProcess は pid のプロセスを TerminateProcess で強制終了する。
func killProcess(pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid)) //nolint:gosec // pid は正の整数に検証済み
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h) //nolint:errcheck // 終了済みハンドルのクローズ失敗は無視してよい
	return windows.TerminateProcess(h, 1)
}
