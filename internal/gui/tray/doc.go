// Package tray は fyne.io/systray のラッパーとトレイ利用可否の判定を提供する。
//
// systray に依存する tray.go / watcher_*.go は gui ビルドタグ配下で、
// トレイ可否の判定ロジック（availability.go）は systray に依存せずタグなしでテストできる（NFR-41）。
package tray
