//go:build gui

package shell

import (
	"github.com/ousiassllc/moleport/internal/i18n"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// trayNoticeID は「トレイで稼働しています」通知の識別子。
const trayNoticeID = "moleport-tray-notice"

// app が Window / Dialog / TrayNoticeSender / settings.Opener を満たすことを確認する。
// Wails の型はこのファイルと app.go の内側に閉じ込める。
var (
	_ Window           = (*app)(nil)
	_ Dialog           = (*app)(nil)
	_ TrayNoticeSender = (*app)(nil)
)

// Show は Window の実装。最小化・非表示のウィンドウを前面に出す（トレイ・2 つ目の起動・認証要求）。
func (a *app) Show() {
	ctx := a.context()
	wailsruntime.WindowUnminimise(ctx)
	wailsruntime.WindowShow(ctx)
	a.notifier.SetVisible(true)
}

// Hide は Window の実装。ウィンドウを非表示にする（トレイ常駐）。
func (a *app) Hide() {
	wailsruntime.WindowHide(a.context())
	a.notifier.SetVisible(false)
}

// Quit は Window の実装。トレイを閉じてから Wails の終了フローを起動する。
func (a *app) Quit() {
	a.stopTray()
	wailsruntime.Quit(a.context())
}

// OpenURL は settings.Opener の実装。既定のブラウザで url を開く（URL の検証は Service 側で行う）。
func (a *app) OpenURL(url string) {
	wailsruntime.BrowserOpenURL(a.context(), url)
}

// ConfirmQuit は Dialog の実装。アクティブな転送が停止される旨の終了確認（トレイ「終了」等のユーザー操作）。
func (a *app) ConfirmQuit(active int) bool {
	return a.askYesNo(
		i18n.T("gui.dialog.quit_confirm_title"),
		i18n.T("gui.dialog.quit_active_confirm", map[string]any{"Count": active}),
	)
}

// ConfirmClose は Dialog の実装。トレイがない環境で閉じる操作を終了確認に倒す（UC-32）。
func (a *app) ConfirmClose(active int) bool {
	key := "gui.dialog.quit_confirm"
	data := map[string]any{}
	if active > 0 {
		key = "gui.dialog.quit_confirm_active"
		data["Count"] = active
	}
	return a.askYesNo(i18n.T("gui.dialog.quit_confirm_title"), i18n.T(key, data))
}

func (a *app) askYesNo(title, message string) bool {
	yes, no := i18n.T("gui.dialog.yes"), i18n.T("gui.dialog.no")
	result, err := wailsruntime.MessageDialog(a.context(), wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         title,
		Message:       message,
		Buttons:       []string{yes, no},
		DefaultButton: no,
		CancelButton:  no,
	})
	return err == nil && IsAffirmative(result, yes)
}

// ShowError は Dialog の実装。起動失敗（引き継ぎ失敗・PID ロック取得失敗を含む）を表示する。
func (a *app) ShowError(key string, cause error) {
	_, _ = wailsruntime.MessageDialog(a.context(), wailsruntime.MessageDialogOptions{
		Type:    wailsruntime.ErrorDialog,
		Title:   i18n.T("gui.dialog.startup_failed_title"),
		Message: i18n.T(key, map[string]any{"Error": cause.Error()}),
	})
}

// initNotifications は OS 通知を初期化する。利用できない環境（D-Bus の通知サービスなし等）では false。
func (a *app) initNotifications() {
	ctx := a.context()
	ok := wailsruntime.InitializeNotifications(ctx) == nil && wailsruntime.IsNotificationAvailable(ctx)
	if ok {
		if granted, err := wailsruntime.RequestNotificationAuthorization(ctx); err != nil || !granted {
			ok = false
		}
	}
	a.notifyOK.Store(ok)
}

// Send は TrayNoticeSender の実装。通知を利用できない環境では false を返す（次回の非表示に持ち越す）。
func (a *app) Send(title, body string) bool {
	if !a.notifyOK.Load() {
		return false
	}
	err := wailsruntime.SendNotification(a.context(), wailsruntime.NotificationOptions{ID: trayNoticeID, Title: title, Body: body})
	return err == nil
}
