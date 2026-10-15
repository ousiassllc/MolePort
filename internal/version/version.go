// Package version はビルド時に埋め込まれるアプリケーションバージョンを保持する。
// CLI (moleport) と GUI (moleport-gui) の両方から参照される。
package version

// Version はビルド時に -ldflags "-X github.com/ousiassllc/moleport/internal/version.Version=<tag>" で設定される。
var Version = "dev"
