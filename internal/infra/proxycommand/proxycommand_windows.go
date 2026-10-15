//go:build windows

package proxycommand

import (
	"log/slog"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// newCommand は cmd /C で command を実行する exec.Cmd を返す。
// command は SSH config 由来の文字列をそのまま cmd.exe に渡す必要があるため、
// Go の引数エスケープを避けて CmdLine を直接指定する（/S で外側の引用符のみ除去される）。
// 新しいプロセスグループで起動し、GUI アプリからコンソールウィンドウが表示されないようにする。
func newCommand(command string) *exec.Cmd {
	cmd := exec.Command("cmd.exe") //nolint:gosec // ProxyCommand は SSH config 由来のユーザー設定値
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       `cmd.exe /S /C "` + command + `"`,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
	return cmd
}

// attachProcess は起動済みプロセスを Job Object に割り当て、終了処理を返す。
// Job Object により、cmd.exe が起動した子孫プロセスも含めて終了できる（NFR-39）。
// Job Object を作成できない場合はプロセス単体の終了にフォールバックする。
// 注意: Start から割り当てまでの間に起動された子孫は Job に含まれない。
func attachProcess(cmd *exec.Cmd) (terminate func()) {
	killProcessOnly := func() { _ = cmd.Process.Kill() }

	job, err := newKillOnCloseJob()
	if err != nil {
		slog.Warn("failed to create job object for ProxyCommand", "error", err)
		return killProcessOnly
	}

	if err := assignToJob(job, cmd.Process.Pid); err != nil {
		slog.Warn("failed to assign ProxyCommand to job object", "error", err)
		_ = windows.CloseHandle(job)
		return killProcessOnly
	}

	return func() {
		_ = windows.TerminateJobObject(job, 1)
		_ = windows.CloseHandle(job)
	}
}

// newKillOnCloseJob はハンドルを閉じたときに所属プロセスを終了する Job Object を作成する。
func newKillOnCloseJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), //nolint:gosec // Windows API の呼び出しに必要
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

// assignToJob は pid のプロセスを job に割り当てる。
func assignToJob(job windows.Handle, pid int) error {
	proc, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid)) //nolint:gosec // pid は起動済みプロセスのもの
	if err != nil {
		return err
	}
	defer windows.CloseHandle(proc) //nolint:errcheck // 割り当て後のハンドルのクローズ失敗は無視してよい
	return windows.AssignProcessToJobObject(job, proc)
}
