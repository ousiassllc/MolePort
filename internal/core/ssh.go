package core

import (
	"context"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHConfigParser は SSH config ファイルを解析しホスト定義を抽出する。
// infra.SSHConfigParser と同じインターフェースで、import cycle を回避するために core で定義する。
type SSHConfigParser interface {
	Parse(configPath string) ([]SSHHost, error)
}

// SSHConnection は SSH 接続とポートフォワーディングの低レベル操作を提供する。
// infra.SSHConnection と同じインターフェースで、import cycle を回避するために core で定義する。
type SSHConnection interface {
	// Dial はホスト情報を使って SSH 接続を確立し、クライアントを返す。
	// cb が nil の場合、SSH エージェントと鍵ファイルのみで認証する。
	// cb が非 nil の場合、パスワード・パスフレーズ・keyboard-interactive 認証も試行する。
	Dial(host SSHHost, cb CredentialCallback) (*ssh.Client, error)

	// Close は SSH 接続を閉じる。
	Close() error

	// LocalForward はローカルポートフォワーディングのリスナーを作成する。
	// localPort でリッスンし、remoteAddr へのトンネルを提供する。
	LocalForward(ctx context.Context, localPort int, remoteAddr string) (net.Listener, error)

	// RemoteForward はリモートポートフォワーディングのリスナーを作成する。
	// リモート側の remotePort でリッスンし、ローカルの localAddr へ転送する。
	RemoteForward(ctx context.Context, remotePort int, localAddr string) (net.Listener, error)

	// DynamicForward は SOCKS5 プロキシとして動作するリスナーを作成する。
	DynamicForward(ctx context.Context, localPort int) (net.Listener, error)

	// IsAlive は SSH 接続が有効かどうかを返す。
	IsAlive() bool

	// KeepAlive は指定間隔で SSH 接続の生存確認を行う。
	// コンテキストがキャンセルされるか、接続が切断されるまでブロックする。
	KeepAlive(ctx context.Context, interval time.Duration)
}

// SSHManager は SSH 接続のライフサイクルを管理する。
type SSHManager interface {
	// LoadHosts は SSH config を解析してホスト一覧を構築・キャッシュし、結果を返す。
	// 呼び出しごとにファイルを再解析するため、副作用としてキャッシュが更新される。
	LoadHosts() ([]SSHHost, error)

	// ReloadHosts は SSH config を再解析し、既存の接続状態を保持したままキャッシュを更新する。
	ReloadHosts() ([]SSHHost, error)

	// GetHosts はキャッシュ済みホスト一覧のコピーを返す。ファイルの再解析は行わない。
	// LoadHosts または ReloadHosts を先に呼び出してキャッシュを構築すること。
	GetHosts() []SSHHost

	// GetHost は名前でホストを検索し返す。見つからない場合はエラーを返す。
	GetHost(name string) (*SSHHost, error)

	// Connect は指定ホストへ SSH 接続を確立する。既に接続中の場合は何もしない。
	Connect(hostName string) error

	// ConnectWithCallback は指定ホストへ SSH 接続を確立する（クレデンシャルコールバック付き）。
	// IPC 経由の接続要求で使用され、パスワード・パスフレーズ・keyboard-interactive 認証をサポートする。
	ConnectWithCallback(hostName string, cb CredentialCallback) error

	// GetPendingAuthHosts は pending_auth 状態のホスト名一覧を返す。
	GetPendingAuthHosts() []string

	// Disconnect は指定ホストとの SSH 接続を切断する。進行中の再接続も停止する。
	Disconnect(hostName string) error

	// IsConnected は指定ホストが現在接続中かを返す。
	IsConnected(hostName string) bool

	// GetConnection は接続済みホストの *ssh.Client を返す。未接続の場合はエラーを返す。
	GetConnection(hostName string) (*ssh.Client, error)

	// GetSSHConnection は接続済みホストの SSHConnection を返す。未接続の場合はエラーを返す。
	GetSSHConnection(hostName string) (SSHConnection, error)

	// Subscribe は SSH イベントを受信するチャネルを返す。
	Subscribe() <-chan SSHEvent

	// Close は全接続を切断し、サブスクライバーチャネルを閉じる。
	Close()
}
