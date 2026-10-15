package service

import (
	"strings"

	"github.com/ousiassllc/moleport/internal/core"
)

// ReleaseURLPrefix はリリースページとして開いてよい URL の接頭辞（UC-35）。
const ReleaseURLPrefix = "https://github.com/ousiassllc/moleport/releases/"

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
	Version VersionSource // 新バージョンのチェック（update.VersionChecker）。nil なら更新なしとして扱う
	Info    Info
}

// VersionSource は新バージョンのチェック結果と有効/無効の切り替え。update.VersionChecker が満たす。
type VersionSource interface {
	// Cached はキャッシュ済みの結果を返す（ネットワークアクセスしない）。無効・未チェックなら ok=false。
	Cached() (core.VersionCheckResult, bool)
	// SetEnabled はチェックの有効/無効を実行中に切り替える。
	SetEnabled(enabled bool)
}

// AvailableUpdate は表示・起動してよい新バージョンを返す。update_check が有効で、キャッシュ済みの結果が
// 新版ありかつリリース URL が ReleaseURLPrefix で始まる場合のみ ok=true（ネットワークアクセスしない）。
func AvailableUpdate(b Backend) (core.VersionCheckResult, bool) {
	if b.Version == nil || b.Config == nil || !b.Config.GetConfig().UpdateCheck.Enabled {
		return core.VersionCheckResult{}, false
	}
	r, ok := b.Version.Cached()
	if !ok || !r.UpdateAvailable || !strings.HasPrefix(r.ReleaseURL, ReleaseURLPrefix) {
		return core.VersionCheckResult{}, false
	}
	return r, true
}

// BackendSource は起動済みの Backend を返す。Shell の Lifecycle が実装する。
// 内蔵デーモンの起動前・起動失敗時は ok=false を返す。
type BackendSource interface {
	Backend() (b Backend, ok bool)
}
