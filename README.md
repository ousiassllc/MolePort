# MolePort

daemon+client アーキテクチャの SSH ポートフォワーディングマネージャ。

`~/.ssh/config` からホスト情報を読み込み、CLI または TUI からポート転送の設定・接続・切断を操作できます。

```
┌──────────────┐     Unix Socket (JSON-RPC 2.0)     ┌──────────────┐
│  CLI / TUI   │ ◄──────────────────────────────────► │   Daemon     │
│  (Client)    │                                      │              │
└──────────────┘                                      │  SSHManager  │
                                                      │  FwdManager  │
                                                      │  ConfigMgr   │
                                                      └──────────────┘
```

## 機能

- **SSH config 連携** --- `~/.ssh/config`（Include 対応）からホストを自動読み込み
- **3種類の転送** --- Local (-L) / Remote (-R) / Dynamic SOCKS5 (-D)
- **リアルタイム監視** --- 接続状態、稼働時間、転送データ量を表示
- **自動再接続** --- 指数バックオフで自動リトライ
- **セッション復元** --- 前回のアクティブ転送を起動時に自動復元
- **daemon+client** --- バックグラウンドデーモンが SSH 接続を管理、CLI/TUI はクライアントとして操作

## 必要環境

- Go 1.23+
- Linux / macOS

## インストール

```bash
git clone https://github.com/ousiassllc/MolePort.git
cd MolePort
make install
```

`make install` は `go install` でバイナリを `$(go env GOPATH)/bin` にインストールします。
PATH に含まれていない場合はシェル設定に追加してください:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

ビルドのみ（`./bin/moleport` に出力）の場合:

```bash
make build
```

## クイックスタート

```bash
# 1. デーモンを起動
moleport daemon start

# 2. SSH ホストに接続
moleport connect prod-server

# 3. 転送ルールを追加
moleport add --host prod-server --type local --local-port 8080 --remote-host localhost --remote-port 80

# 4. フォワーディングを開始
moleport start web

# 5. TUI ダッシュボードで監視
moleport tui
```

## CLI コマンド

| コマンド | 説明 |
|---------|------|
| `moleport daemon start` | デーモンをバックグラウンドで起動 |
| `moleport daemon stop [--purge]` | デーモンを停止（`--purge`: 状態クリア） |
| `moleport daemon status` | デーモンの稼働状態を表示 |
| `moleport connect <host>` | SSH ホストに接続 |
| `moleport disconnect <host>` | SSH ホストを切断 |
| `moleport add [flags]` | 転送ルールを追加 |
| `moleport delete <name>` | 転送ルールを削除 |
| `moleport start <name>` | フォワーディングを開始 |
| `moleport stop <name> / --all` | フォワーディングを停止（`--all`: 全停止） |
| `moleport list [--json]` | ホスト・転送ルールの一覧 |
| `moleport status [name]` | 接続状態のサマリー |
| `moleport config [--json]` | 設定を表示 |
| `moleport reload` | SSH config を再読み込み |
| `moleport tui` | TUI ダッシュボードを起動 |
| `moleport version` | バージョン情報を表示 |
| `moleport help` | ヘルプを表示 |

## TUI キーバインド

| キー | 動作 |
|------|------|
| `↑`/`k` `↓`/`j` | 項目を選択 |
| `Enter` | 転送の接続/切断トグル |
| `Tab` | ペイン切り替え |
| `d` | 選択中の転送を切断 |
| `x` | 選択中の転送を削除 |
| `/` | コマンド入力にフォーカス |
| `?` | ヘルプ表示 |
| `Esc` | キャンセル |
| `q` / `Ctrl+C` | 終了 |

## アーキテクチャ

```
CLI / TUI (Client)
  ↕  Unix Socket (JSON-RPC 2.0)
Daemon
  ├── IPC Server (EventBroker + Handler)
  ├── Core Layer
  │     SSHManager / ForwardManager / ConfigManager
  └── Infrastructure Layer
        SSHConnection / SSHConfigParser / YAMLStore
```

## 設定

設定ファイル: `~/.config/moleport/config.yaml`

```yaml
ssh_config_path: "~/.ssh/config"

reconnect:
  enabled: true
  max_retries: 10
  initial_delay: "1s"
  max_delay: "60s"

session:
  auto_restore: true

log:
  level: "info"
  file: "~/.config/moleport/moleport.log"
```

## 開発

```bash
make help       # 利用可能なターゲットを表示
make build      # ビルド
make test       # テスト実行
make test-race  # race detector 付きテスト
make vet        # go vet
make fmt        # go fmt
```

## ライセンス

MIT
