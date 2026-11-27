// Package daemon はバックグラウンドデーモンプロセスのライフサイクル管理を提供する。
// コンポーネントの組み立て、起動・停止、状態の永続化を担う。
// PID ファイル管理は pidfile、デーモンの fork・接続確保・引き継ぎは control サブパッケージに置く。
package daemon
