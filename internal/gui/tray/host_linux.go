//go:build gui && linux

package tray

import "github.com/godbus/dbus/v5"

// watcherName は fyne.io/systray が RegisterStatusNotifierItem を呼ぶ相手（StatusNotifierWatcher）。
const watcherName = "org.kde.StatusNotifierWatcher"

// hostCheck は Linux でセッションバスに StatusNotifierWatcher が存在するかを確認する。
// セッションバスに接続できない場合（D-Bus なし）もトレイ不可として扱う。
func hostCheck() (required, present bool) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return true, false
	}
	defer func() { _ = conn.Close() }()

	var has bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, watcherName).Store(&has); err != nil {
		return true, false
	}
	return true, has
}
