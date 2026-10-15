package service

import "github.com/ousiassllc/moleport/internal/core"

// Info は内蔵デーモンの実行情報（ヘッダー表示用）。
type Info struct {
	Version string
	PID     int
}

// Backend は Service 層が操作する Core の参照。内蔵デーモンの Start 後にのみ存在する。
type Backend struct {
	SSH     core.SSHManager
	Forward core.ForwardManager
	Config  core.ConfigManager
	Info    Info
}

// BackendSource は起動済みの Backend を返す。Shell の Lifecycle が実装する。
// 内蔵デーモンの起動前・起動失敗時は ok=false を返す。
type BackendSource interface {
	Backend() (b Backend, ok bool)
}
