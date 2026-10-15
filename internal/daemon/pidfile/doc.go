// Package pidfile はデーモンの PID ファイルと OS ファイルロックによるプロセス排他を提供する。
// ロックとプロセス操作は OS ごとに pidfile_unix.go / pidfile_windows.go で実装する。
package pidfile
