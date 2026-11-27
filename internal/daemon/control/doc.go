// Package control は稼働中デーモンの外部制御（デーモンプロセスの fork・起動確認と接続・
// headless デーモンの引き継ぎ）を提供する。Linterly の 2000 行/ディレクトリ制限のため `daemon` から分離している。
package control
