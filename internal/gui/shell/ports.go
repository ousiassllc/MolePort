package shell

// Window は Lifecycle / CloseController が操作するウィンドウ。Wails の型を持たず、
// Shell（app.go）が runtime.* で実装する。
type Window interface {
	// Show はウィンドウを最小化解除して前面に出す。
	Show()
	// Hide はウィンドウを非表示にする（トレイ常駐。プロセスと転送は継続）。
	Hide()
	// Quit は Wails の終了フロー（runtime.Quit）を起動する。
	Quit()
}

// Dialog はユーザーへの確認・エラー表示。Shell が runtime.MessageDialog で実装する。
type Dialog interface {
	// ConfirmQuit は「アクティブな転送 active 件が停止されます」の終了確認を出し、承認されたら true を返す。
	// トレイ「終了」等のユーザー操作の終了（UC-36）に使う。
	ConfirmQuit(active int) bool
	// ConfirmClose はトレイが使えない環境でウィンドウを閉じる操作に対する終了確認（UC-32）を出す。
	// active が 0 のときは転送停止の文言を含めない。承認されたら true を返す。
	ConfirmClose(active int) bool
	// ShowError は起動失敗のエラーダイアログを表示する。key は StartupErrorKey が返す i18n キー。
	ShowError(key string, cause error)
}

// TrayNoticeSender は OS 通知を送る。Shell が runtime.SendNotification で実装する。
type TrayNoticeSender interface {
	// Send は通知を 1 件送る。通知を利用できない環境では false を返す。
	Send(title, body string) bool
}
