// Package shell は GUI の Shell 層。Wails ランタイムの起動・ウィンドウ制御・内蔵デーモンの
// ライフサイクルを統括する。
//
// このパッケージのうち Wails に依存するファイル（app.go / window.go）だけが gui ビルドタグ配下で、
// それ以外（Lifecycle・ウィンドウ閉じる操作の決定表・可視性ゲート付き Notifier）は
// Wails 型を一切持たず、タグなしの go test ./... でテストできる（NFR-41）。
package shell
