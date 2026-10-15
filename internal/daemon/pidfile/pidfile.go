package pidfile

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// PIDFile はデーモンの PID ファイルを管理する。
// OS のファイルロック（Unix: flock / Windows: LockFileEx）によるプロセス排他を提供する。
// ロックとプロセス操作の OS 依存部分は pidfile_unix.go / pidfile_windows.go にある。
type PIDFile struct {
	path string
	file *os.File
}

// NewPIDFile は指定パスの PIDFile を生成する。
func NewPIDFile(path string) *PIDFile {
	return &PIDFile{path: path}
}

// Acquire は PID ファイルを作成し、排他ロックを取得する。
// ロック取得に失敗した場合（デーモンが既に起動中）はエラーを返す。
func (p *PIDFile) Acquire() error {
	f, err := os.OpenFile(p.path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open pid file: %w", err)
	}

	if err := lockFile(f); err != nil {
		f.Close()
		return fmt.Errorf("daemon already running (lock failed): %w", err)
	}

	if err := f.Truncate(0); err != nil {
		_ = unlockFile(f)
		f.Close()
		return fmt.Errorf("truncate pid file: %w", err)
	}
	// OpenFile 直後のためオフセットは 0。Seek は不要。

	pid := os.Getpid()
	if _, err := fmt.Fprintf(f, "%d\n", pid); err != nil {
		_ = unlockFile(f)
		f.Close()
		return fmt.Errorf("write pid: %w", err)
	}

	p.file = f
	return nil
}

// Release は PID ファイルを削除し、ロックを解放してファイルを閉じる。
// 複数回呼び出しても安全（冪等）。
func (p *PIDFile) Release() error {
	if p.file == nil {
		return nil
	}

	err := releaseFile(p.path, p.file)
	p.file = nil
	return err
}

// readPID は PID ファイルから PID を読み取る。
// ファイルが存在しない、または内容が不正な場合は (0, false) を返す。
func readPID(path string) (int, bool) {
	data, err := os.ReadFile(path) //nolint:gosec // path は内部で生成された PID ファイルパス
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

// KillProcess は PID ファイルから PID を読み取り、強制終了し、PID ファイルを削除する。
// Unix では SIGKILL、Windows では TerminateProcess を使用する。
// PID ファイルが存在しない場合やプロセスが既に終了している場合もエラーを返す。
func KillProcess(pidPath string) error {
	data, err := os.ReadFile(pidPath) //nolint:gosec // pidPath は内部で生成された PID ファイルパス
	if err != nil {
		return fmt.Errorf("read pid file: %w", err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		_ = os.Remove(pidPath)
		return fmt.Errorf("invalid pid file content: %q", pidStr)
	}

	if err := killProcess(pid); err != nil {
		_ = os.Remove(pidPath)
		return fmt.Errorf("kill process %d: %w", pid, err)
	}

	if err := os.Remove(pidPath); err != nil {
		return fmt.Errorf("remove pid file: %w", err)
	}

	return nil
}

// IsRunning は PID ファイルを読み取り、対応するプロセスが実行中かを返す。
// ファイルが存在しない、内容が不正、またはプロセスが存在しない場合は (false, 0) を返す。
// 注意: プロセス生存確認は PID の存在のみを確認する。PID 再利用により偽陽性の可能性があるが、
// Acquire() のファイルロックが実際の排他制御を保証する。
// 引き継ぎ判定のように PID 再利用の影響を避けたい場合は IsLocked を使うこと。
func IsRunning(path string) (bool, int) {
	pid, ok := readPID(path)
	if !ok || !processExists(pid) {
		return false, 0
	}
	return true, pid
}

// IsLocked は PID ファイルの排他ロックが他プロセスに保持されているかを非破壊で判定する。
// ロックを試行して即解放するため、PID ファイルの内容・存在は変更しない。
// ファイルが存在しない、または誰もロックを保持していない（stale PID ファイル）場合は (false, 0) を返す。
// ロック保持中の場合は PID ファイルに記録された PID を返す（読み取れなければ 0）。
// 注意: 同一プロセス内の別ファイルディスクリプタが保持するロックも「保持中」と判定される。
func IsLocked(path string) (bool, int) {
	f, err := os.OpenFile(path, os.O_RDWR, 0) //nolint:gosec // path は内部で生成された PID ファイルパス
	if err != nil {
		return false, 0
	}
	defer func() { _ = f.Close() }()

	err = lockFile(f)
	if err == nil {
		_ = unlockFile(f)
		return false, 0
	}
	if !isLockHeld(err) {
		return false, 0
	}

	pid, _ := readPID(path)
	return true, pid
}
