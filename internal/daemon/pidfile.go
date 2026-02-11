package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// PIDFile はデーモンの PID ファイルを管理する。
// flock によるプロセス排他を提供する。
type PIDFile struct {
	path string
	file *os.File
}

// NewPIDFile は指定パスの PIDFile を生成する。
func NewPIDFile(path string) *PIDFile {
	return &PIDFile{path: path}
}

// Acquire は PID ファイルを作成し、flock で排他ロックを取得する。
// ロック取得に失敗した場合（デーモンが既に起動中）はエラーを返す。
func (p *PIDFile) Acquire() error {
	f, err := os.OpenFile(p.path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open pid file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return fmt.Errorf("daemon already running (lock failed): %w", err)
	}

	if err := f.Truncate(0); err != nil {
		f.Close()
		return fmt.Errorf("truncate pid file: %w", err)
	}
	// OpenFile 直後のためオフセットは 0。Seek は不要。

	pid := os.Getpid()
	if _, err := fmt.Fprintf(f, "%d\n", pid); err != nil {
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

	os.Remove(p.path)
	syscall.Flock(int(p.file.Fd()), syscall.LOCK_UN)
	err := p.file.Close()
	p.file = nil
	return err
}

// IsRunning は PID ファイルを読み取り、対応するプロセスが実行中かを返す。
// ファイルが存在しない、内容が不正、またはプロセスが存在しない場合は (false, 0) を返す。
// 注意: Kill(pid, 0) はプロセスの存在のみを確認する。PID 再利用により偽陽性の可能性があるが、
// Acquire() の flock が実際の排他制御を保証する。
func IsRunning(path string) (bool, int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, 0
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return false, 0
	}

	if err := syscall.Kill(pid, 0); err != nil {
		return false, 0
	}

	return true, pid
}
