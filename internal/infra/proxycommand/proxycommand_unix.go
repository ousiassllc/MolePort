//go:build !windows

package proxycommand

import (
	"os/exec"
	"syscall"
)

// newCommand は sh -c で command を実行する exec.Cmd を返す。
// 子孫プロセスをまとめて終了できるよう、新しいプロセスグループで起動する。
func newCommand(command string) *exec.Cmd {
	cmd := exec.Command("sh", "-c", command) //nolint:gosec // ProxyCommand は SSH config 由来のユーザー設定値
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// attachProcess は起動済みプロセスの終了処理を返す。
// プロセスグループ全体（子孫を含む）に SIGKILL を送る。
func attachProcess(cmd *exec.Cmd) (terminate func()) {
	pid := cmd.Process.Pid
	return func() {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}
