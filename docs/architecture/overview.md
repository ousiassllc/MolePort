# MolePort アーキテクチャ設計

## 概要

MolePort は Go で実装される SSH ポートフォワーディング管理ツールである。
バックグラウンドデーモンが SSH 接続とポートフォワーディングを永続管理し、CLI サブコマンドおよび TUI ダッシュボードがデーモンのクライアントとして動作する。
デーモンとクライアント間は Unix ドメインソケット上の JSON-RPC 2.0 プロトコルで通信する。

## 技術選定

| カテゴリ | 技術 | バージョン | 選定理由 |
|---------|------|-----------|---------|
| 言語 | Go | 1.23+ | シングルバイナリ配布、goroutine による並行処理、クロスコンパイル |
| TUI フレームワーク | [Bubble Tea](https://github.com/charmbracelet/bubbletea) | v1.x | Elm Architecture、エコシステム充実、活発なメンテナンス |
| TUI スタイリング | [Lip Gloss](https://github.com/charmbracelet/lipgloss) | v1.x | Bubble Tea との統合、宣言的スタイリング |
| TUI コンポーネント | [Bubbles](https://github.com/charmbracelet/bubbles) | v1.x | テキスト入力、リスト、テーブル等のウィジェット |
| SSH | [x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh) | latest | Go 標準拡張、外部依存なし、接続の完全制御 |
| SSH config 解析 | [ssh_config](https://github.com/kevinburke/ssh_config) | v1.x | SSH config の完全な解析（Include 対応） |
| YAML | [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) | v3 | 設定ファイルの読み書き |
| ログ | [log/slog](https://pkg.go.dev/log/slog) | stdlib | Go 標準の構造化ログ |
| IPC | JSON-RPC 2.0 over Unix Domain Socket | — | 標準化された RPC プロトコル。リクエスト/レスポンス/通知の明確な区別。デバッグ容易 |

## 全体構成図

```mermaid
graph TB
    subgraph Clients["クライアント"]
        CLI["CLI<br/>moleport &lt;subcommand&gt;"]
        TUI["TUI<br/>moleport tui"]
    end

    Socket["Unix Domain Socket<br/>~/.config/moleport/moleport.sock<br/>JSON-RPC 2.0"]

    subgraph Daemon["デーモンプロセス (moleport daemon start)"]
        IPCServer["IPC Server<br/>JSON-RPC Handler"]

        subgraph Core["Core Layer"]
            SSHManager["SSHManager"]
            ForwardManager["ForwardManager"]
            ConfigManager["ConfigManager"]
        end

        subgraph Infra["Infrastructure Layer"]
            SSHConn["SSHConnection<br/>(x/crypto/ssh)"]
            SSHConfigParser["SSHConfigParser"]
            YAMLStore["YAMLStore"]
        end

        IPCServer --> SSHManager
        IPCServer --> ForwardManager
        IPCServer --> ConfigManager
        SSHManager --> SSHConn
        SSHManager --> SSHConfigParser
        ConfigManager --> YAMLStore
    end

    CLI --> Socket
    TUI --> Socket
    Socket --> IPCServer
```

## アーキテクチャの変更点（v1 → v2）

### v1（現行: モノリシック TUI）

```
MolePort プロセス = TUI + Core + Infra（単一プロセス）
```

- TUI が直接 Core Layer を呼び出す
- プロセス終了 = 全接続終了
- TUI を閉じるとポートフォワーディングも停止

### v2（新設計: デーモン + クライアント）

```
デーモンプロセス = IPC Server + Core + Infra（常駐）
クライアント    = CLI / TUI（必要時に起動・終了）
```

- デーモンがバックグラウンドで常駐し、SSH 接続を維持
- CLI/TUI はクライアントとして接続し、操作・監視を行う
- TUI を閉じてもポートフォワーディングは継続
- 複数クライアントが同時接続可能

## デーモンプロセス

### ライフサイクル

```mermaid
stateDiagram-v2
    [*] --> Stopped

    Stopped --> Starting : moleport daemon start
    Starting --> Running : ソケット Listen 開始
    Starting --> Stopped : 起動失敗（ポート競合等）
    Running --> Stopping : moleport daemon stop / SIGTERM / SIGINT
    Stopping --> Stopped : グレースフルシャットダウン完了

    Running --> Running : クライアント接続/切断
```

### 起動シーケンス

```mermaid
sequenceDiagram
    actor User
    participant CLI as moleport daemon start
    participant Fork as フォークされたプロセス
    participant Daemon as デーモンプロセス

    User->>CLI: moleport daemon start
    CLI->>CLI: 既存デーモンの確認（PID ファイル + ソケット疎通）
    alt デーモン稼働中
        CLI-->>User: "デーモンは既に稼働中です (PID: xxxx)"
    else デーモン未稼働
        CLI->>Fork: os.StartProcess (自身を --daemon-mode で再起動)
        Fork->>Fork: setsid() でセッションリーダーに
        Fork->>Fork: stdin/stdout/stderr を /dev/null にリダイレクト
        Fork->>Daemon: 初期化開始
        Daemon->>Daemon: PID ファイル作成 (~/.config/moleport/moleport.pid)
        Daemon->>Daemon: config.yaml 読み込み
        Daemon->>Daemon: Unix ソケット Listen 開始
        Daemon->>Daemon: state.yaml からセッション復元
        Daemon->>Daemon: auto_connect ルールの自動接続
        Fork-->>CLI: プロセス起動成功
        CLI-->>User: "デーモンを起動しました (PID: xxxx)"
    end
```

### 停止シーケンス

```mermaid
sequenceDiagram
    actor User
    participant CLI as moleport daemon stop
    participant Daemon as デーモンプロセス

    User->>CLI: moleport daemon stop
    CLI->>Daemon: JSON-RPC: daemon.shutdown
    Daemon->>Daemon: 全クライアントに shutdown 通知
    Daemon->>Daemon: 全ポートフォワーディング停止
    Daemon->>Daemon: 全 SSH 接続切断
    Daemon->>Daemon: state.yaml に状態保存
    Daemon->>Daemon: Unix ソケット Close
    Daemon->>Daemon: PID ファイル削除
    Daemon-->>CLI: shutdown 応答
    CLI-->>User: "デーモンを停止しました"
```

### PID ファイル管理

- パス: `~/.config/moleport/moleport.pid`
- 起動時に PID ファイルを作成（排他ロック `flock`）
- 既存 PID ファイルがある場合は、プロセスの生存確認を行う
  - 生存中 → 起動を拒否
  - 死亡済み（stale PID）→ PID ファイルを削除して起動続行
- 終了時に PID ファイルを削除

## IPC 通信

### 概要

クライアント（CLI/TUI）とデーモン間は Unix ドメインソケット上で JSON-RPC 2.0 プロトコルにより通信する。

- **ソケットパス**: `~/.config/moleport/moleport.sock`
- **プロトコル**: JSON-RPC 2.0（改行区切り NDJSON）
- **方向**:
  - クライアント → デーモン: リクエスト（`method` + `params`）
  - デーモン → クライアント: レスポンス（`result` / `error`）
  - デーモン → クライアント: 通知（`method` + `params`, `id` なし）

### 通信パターン

#### パターン1: 同期リクエスト/レスポンス（CLI 向け）

```mermaid
sequenceDiagram
    participant CLI as CLI クライアント
    participant Daemon as デーモン

    CLI->>Daemon: {"jsonrpc":"2.0","id":1,"method":"forward.list","params":{}}
    Daemon-->>CLI: {"jsonrpc":"2.0","id":1,"result":[...]}
```

CLI は1回のリクエスト/レスポンスで完結し、接続を切断する。

#### パターン2: サブスクリプション（TUI 向け）

```mermaid
sequenceDiagram
    participant TUI as TUI クライアント
    participant Daemon as デーモン

    TUI->>Daemon: {"jsonrpc":"2.0","id":1,"method":"events.subscribe","params":{"types":["forward","ssh","metrics"]}}
    Daemon-->>TUI: {"jsonrpc":"2.0","id":1,"result":{"subscription_id":"sub-1"}}

    Note over Daemon,TUI: 以降、状態変化時にデーモンから通知が送られる

    Daemon-->>TUI: {"jsonrpc":"2.0","method":"event.forward","params":{"type":"started","rule":"prod-web",...}}
    Daemon-->>TUI: {"jsonrpc":"2.0","method":"event.metrics","params":{"sessions":[...]}}
    Daemon-->>TUI: {"jsonrpc":"2.0","method":"event.ssh","params":{"type":"disconnected","host":"prod-server",...}}

    TUI->>Daemon: {"jsonrpc":"2.0","id":2,"method":"events.unsubscribe","params":{"subscription_id":"sub-1"}}
    Daemon-->>TUI: {"jsonrpc":"2.0","id":2,"result":{"ok":true}}
```

TUI は `events.subscribe` でイベントストリームを開始し、接続中はデーモンから通知を受け続ける。

### JSON-RPC メソッド一覧

| メソッド | 方向 | 説明 |
|---------|------|------|
| `host.list` | req/res | SSH ホスト一覧を取得 |
| `host.reload` | req/res | SSH config を再読み込み |
| `ssh.connect` | req/res | SSH ホストに接続 |
| `ssh.disconnect` | req/res | SSH ホストを切断 |
| `forward.list` | req/res | 転送ルール一覧を取得 |
| `forward.add` | req/res | 転送ルールを追加 |
| `forward.delete` | req/res | 転送ルールを削除 |
| `forward.start` | req/res | ポートフォワーディングを開始 |
| `forward.stop` | req/res | ポートフォワーディングを停止 |
| `session.list` | req/res | アクティブセッション一覧を取得 |
| `session.get` | req/res | セッション詳細を取得 |
| `config.get` | req/res | 設定を取得 |
| `config.update` | req/res | 設定を更新 |
| `daemon.status` | req/res | デーモンの状態を取得 |
| `daemon.shutdown` | req/res | デーモンを停止 |
| `events.subscribe` | req/res | イベントストリームを開始 |
| `events.unsubscribe` | req/res | イベントストリームを停止 |
| `credential.request` | notification | クレデンシャル入力要求（デーモン → クライアント） |
| `credential.response` | req/res | クレデンシャル入力応答（クライアント → デーモン） |
| `event.ssh` | notification | SSH 状態変化通知 |
| `event.forward` | notification | 転送状態変化通知 |
| `event.metrics` | notification | メトリクス更新通知 |

## レイヤー構造

### IPC Layer（通信層）— 新規

- **責務**: JSON-RPC 2.0 メッセージのシリアライズ/デシリアライズ、ルーティング、イベント配信
- **設計方針**: プロトコルの詳細を隠蔽し、Core Layer とクライアントを疎結合にする
- **主要コンポーネント**:
  - `IPCServer`: Unix ソケット上で JSON-RPC リクエストを受け付け、ハンドラにディスパッチ
  - `IPCClient`: CLI/TUI が使用するクライアントライブラリ。メソッド呼び出しとイベント受信を提供
  - `EventBroker`: サブスクリプション管理とイベント配信

### Core Layer（ビジネスロジック層）

- **責務**: SSH 接続管理、ポートフォワーディング制御、設定管理
- **設計方針**: IPC に依存しない純粋なロジック。テスト容易性を確保する
- **主要コンポーネント**:
  - `SSHManager`: SSH 接続のライフサイクル管理（接続、切断、再接続）
  - `ForwardManager`: ポートフォワーディングルールの管理と実行
  - `ConfigManager`: 設定ファイルと状態ファイルの読み書き
- **変更なし**: v1 の Core Layer をそのまま活用。TUI 依存がないため移行がスムーズ

### Infrastructure Layer（インフラ層）

- **責務**: 外部リソースとのやり取り（SSH 接続、ファイル I/O）
- **設計方針**: Core Layer から interface 経由で利用される
- **主要コンポーネント**:
  - `SSHConnection`: `x/crypto/ssh` のラッパー
  - `SSHConfigParser`: SSH config ファイルの解析
  - `YAMLStore`: YAML ファイルの読み書き
- **変更なし**: v1 の Infrastructure Layer をそのまま活用

### TUI Layer（プレゼンテーション層 — Atomic Design）

- **責務**: ユーザー入力の受け付け、画面描画
- **設計方針**: Bubble Tea の Model-Update-View パターン + Atomic Design
- **変更点**: Core Layer の直接呼び出しから IPC Client 経由に変更
  - `MainModel` が `IPCClient` を保持
  - コマンド実行 → IPC リクエスト
  - イベント受信 → Bubble Tea Msg に変換して UI 更新

#### Atomic Design コンポーネント階層図

```mermaid
graph TD
    subgraph Page
        Dashboard["DashboardPage"]
    end

    subgraph Organisms
        SP["SetupPanel"]
        FP["ForwardPanel"]
        LP["LogPanel"]
        SB["StatusBar"]
    end

    subgraph Molecules
        HR["HostRow"]
        FR["ForwardRow"]
        PI["PromptInput"]
        CD["ConfirmDialog"]
        PW["PasswordInput"]
    end

    subgraph Atoms
        Badge["StatusBadge"]
        Port["PortLabel"]
        Spin["Spinner"]
        Key["KeyHint"]
        DS["DataSize"]
        Dur["Duration"]
        Div["Divider"]
    end

    Dashboard --> SP
    Dashboard --> FP
    Dashboard --> LP
    Dashboard --> SB

    SP --> HR
    SP --> Div
    FP --> FR
    FP --> Div
    LP --> PI
    SB --> Key

    HR --> Badge
    HR --> Port
    FR --> Port
    FR --> Badge
    FR --> Dur
    FR --> DS
    PI --> Key
    CD --> Key
    PW --> Key
```

### CLI Layer（コマンドライン層）— 新規

- **責務**: CLI サブコマンドの解析と実行
- **設計方針**: 各サブコマンドが IPC Client を介してデーモンに操作を要求し、結果を表示する
- **主要コンポーネント**:
  - `CLIRouter`: サブコマンドの解析とディスパッチ（Go 標準の `flag` パッケージ）
  - 各サブコマンドハンドラ: `daemon`, `connect`, `disconnect`, `add`, `delete`, `start`, `stop`, `list`, `status`, `config`, `reload`, `tui`, `help`, `version`

## ディレクトリ構成

```
moleport/
├── cmd/
│   └── moleport/
│       └── main.go                  # エントリポイント（CLI ルーター）
├── internal/
│   ├── daemon/                      # デーモンプロセス
│   │   ├── daemon.go                # Daemon（起動・停止・ライフサイクル管理）
│   │   ├── ensure.go                # デーモン起動確認・IPC 接続ヘルパー
│   │   ├── fork.go                  # フォーク処理（self-fork）
│   │   └── pidfile.go               # PID ファイル管理
│   ├── ipc/                         # IPC 通信層
│   │   ├── server.go                # IPCServer（JSON-RPC サーバー）
│   │   ├── client.go                # IPCClient（JSON-RPC クライアント）
│   │   ├── handler.go               # RPC メソッドハンドラ
│   │   ├── handler_convert.go       # コアエラー・型の RPC 変換
│   │   ├── broker.go                # EventBroker（イベント配信）
│   │   └── protocol.go              # JSON-RPC メッセージ型定義
│   ├── cli/                         # CLI サブコマンド
│   │   ├── root.go                  # CLIRouter（サブコマンド解析）
│   │   ├── credential.go            # CLI 用クレデンシャルハンドラ
│   │   ├── daemon_cmd.go            # moleport daemon start/stop/status
│   │   ├── connect_cmd.go           # moleport connect <host>
│   │   ├── disconnect_cmd.go        # moleport disconnect <host>
│   │   ├── add_cmd.go               # moleport add
│   │   ├── delete_cmd.go            # moleport delete <name>
│   │   ├── start_cmd.go             # moleport start
│   │   ├── stop_cmd.go              # moleport stop
│   │   ├── list_cmd.go              # moleport list
│   │   ├── status_cmd.go            # moleport status
│   │   ├── config_cmd.go            # moleport config
│   │   ├── reload_cmd.go            # moleport reload
│   │   ├── help_cmd.go              # moleport help
│   │   ├── version_cmd.go           # moleport version
│   │   └── tui_cmd.go               # moleport tui
│   ├── tui/                         # TUI Layer（Atomic Design）
│   │   ├── app/
│   │   │   ├── app.go               # MainModel（IPCClient 経由に変更）
│   │   │   └── convert.go           # IPC/コア型変換
│   │   ├── styles.go
│   │   ├── keys.go
│   │   ├── messages.go
│   │   ├── atoms/
│   │   ├── molecules/
│   │   ├── organisms/
│   │   └── pages/
│   ├── core/                        # Core Layer
│   │   ├── ssh.go
│   │   ├── forward.go
│   │   ├── config.go
│   │   ├── socks5.go                # SOCKS5 プロキシ
│   │   └── types.go
│   └── infra/                       # Infrastructure Layer
│       ├── sshconn.go
│       ├── sshconfig.go
│       ├── auth.go
│       ├── yamlstore.go
│       └── util.go
├── go.mod
├── go.sum
└── docs/
```

## 通信フロー

### CLI からのポートフォワーディング開始

```mermaid
sequenceDiagram
    actor User
    participant CLI as CLI (moleport connect prod-server)
    participant Daemon as デーモン (IPCServer)
    participant Core as Core Layer
    participant Infra as Infra Layer
    participant Remote as SSH Server

    User->>CLI: moleport connect prod-server
    CLI->>Daemon: JSON-RPC: ssh.connect {"host":"prod-server"}
    Daemon->>Core: SSHManager.Connect("prod-server")
    Core->>Infra: Dial(host)
    Infra->>Remote: SSH Handshake
    Remote-->>Infra: 認証成功
    Infra-->>Core: Connection established
    Core->>Core: auto_connect ルールのフォワーディング開始
    Core-->>Daemon: 接続完了
    Daemon-->>CLI: {"result":{"status":"connected","host":"prod-server"}}
    CLI-->>User: "prod-server に接続しました (2 forwards started)"
```

### クレデンシャルコールバック（パスワード認証の例）

```mermaid
sequenceDiagram
    actor User
    participant CLI as CLI (moleport connect prod-server)
    participant Daemon as デーモン (IPCServer)
    participant Core as Core Layer
    participant Infra as Infra Layer
    participant Remote as SSH Server

    User->>CLI: moleport connect prod-server
    CLI->>Daemon: JSON-RPC: ssh.connect {"host":"prod-server"}
    Daemon->>Core: SSHManager.Connect("prod-server")
    Core->>Infra: Dial(host)
    Infra->>Remote: SSH Handshake 開始

    Note over Infra,Remote: エージェント・鍵認証が失敗

    Remote-->>Infra: パスワード認証を要求
    Infra-->>Core: CredentialRequest{type:"password", host:"prod-server"}
    Core-->>Daemon: CredentialRequest イベント
    Daemon-->>CLI: credential.request {"request_id":"cr-1","type":"password","host":"prod-server","prompt":"Password:"}

    CLI->>User: パスワード入力プロンプト（エコーなし）
    User->>CLI: パスワードを入力
    CLI->>Daemon: credential.response {"request_id":"cr-1","value":"****"}
    Daemon->>Core: CredentialResponse 転送
    Core->>Infra: パスワードを認証メソッドに渡す

    Infra->>Remote: パスワードで認証
    Remote-->>Infra: 認証成功
    Infra-->>Core: Connection established
    Core->>Core: auto_connect ルールのフォワーディング開始
    Core-->>Daemon: 接続完了
    Daemon-->>CLI: {"result":{"status":"connected","host":"prod-server"}}
    CLI-->>User: "prod-server に接続しました"
```

### クレデンシャルコールバック（keyboard-interactive 複数チャレンジの例）

```mermaid
sequenceDiagram
    actor User
    participant Client as CLI / TUI
    participant Daemon as デーモン
    participant Remote as SSH Server

    Client->>Daemon: ssh.connect {"host":"2fa-server"}

    Note over Daemon,Remote: keyboard-interactive 認証開始

    Remote-->>Daemon: Challenge 1: "Password:"
    Daemon-->>Client: credential.request {"request_id":"cr-1","type":"keyboard-interactive","prompts":[{"prompt":"Password:","echo":false}]}
    Client->>User: "Password:" プロンプト
    User->>Client: パスワード入力
    Client->>Daemon: credential.response {"request_id":"cr-1","answers":["****"]}

    Remote-->>Daemon: Challenge 2: "OTP Code:"
    Daemon-->>Client: credential.request {"request_id":"cr-2","type":"keyboard-interactive","prompts":[{"prompt":"OTP Code:","echo":true}]}
    Client->>User: "OTP Code:" プロンプト
    User->>Client: OTP コード入力
    Client->>Daemon: credential.response {"request_id":"cr-2","answers":["123456"]}

    Remote-->>Daemon: 認証成功
    Daemon-->>Client: {"result":{"status":"connected"}}
```

### セッション復元時の pending_auth フロー

```mermaid
sequenceDiagram
    participant Daemon as デーモン起動時
    participant State as state.yaml

    Daemon->>State: 前回アクティブ転送を読み込み
    State-->>Daemon: [prod-server, 2fa-server]

    Note over Daemon: prod-server: エージェント認証 → 成功
    Daemon->>Daemon: prod-server を Connected に

    Note over Daemon: 2fa-server: エージェント認証 → 失敗（パスワード必要）
    Daemon->>Daemon: 2fa-server を PendingAuth に

    Note over Daemon: クライアント接続時

    participant TUI as TUI クライアント
    TUI->>Daemon: events.subscribe
    Daemon-->>TUI: event.ssh {"type":"pending_auth","host":"2fa-server"}
    TUI->>TUI: ホスト一覧に "⏳ Pending Auth" を表示

    Note over TUI: ユーザーが connect を実行
    TUI->>Daemon: ssh.connect {"host":"2fa-server"}
    Note over Daemon,TUI: UC-13 クレデンシャルコールバックフローへ
```

### TUI のリアルタイム更新

```mermaid
sequenceDiagram
    participant TUI as TUI (moleport tui)
    participant Daemon as デーモン
    participant Core as Core Layer

    TUI->>Daemon: events.subscribe {"types":["ssh","forward","metrics"]}
    Daemon-->>TUI: {"subscriptionId":"sub-1"}

    Note over Daemon: SSH 接続断を検知
    Core-->>Daemon: SSHEvent{Disconnected}
    Daemon-->>TUI: event.ssh {"type":"disconnected","host":"prod-server"}
    TUI->>TUI: ダッシュボード更新（状態 → Reconnecting）

    Note over Daemon: 再接続成功
    Core-->>Daemon: SSHEvent{Connected}
    Daemon-->>TUI: event.ssh {"type":"connected","host":"prod-server"}
    TUI->>TUI: ダッシュボード更新（状態 → Connected）
```

## 並行処理モデル

```mermaid
graph TD
    subgraph Daemon["デーモンプロセス"]
        Main["Main goroutine<br/>シグナルハンドリング"]
        IPC["goroutine: IPC Server<br/>Accept ループ"]

        IPC --> Client1["goroutine: Client #1 (CLI)"]
        IPC --> Client2["goroutine: Client #2 (TUI)"]

        Main --> Conn1["goroutine: SSH Connection #1"]
        Main --> Conn2["goroutine: SSH Connection #2"]
        Main --> Monitor["goroutine: Reconnect Monitor"]
        Main --> Metrics["goroutine: Metrics Collector"]
        Main --> Broker["goroutine: Event Broker"]

        Conn1 --> Fwd1["goroutine: Local Forward<br/>:8080 → remote:80"]
        Conn1 --> Fwd2["goroutine: Local Forward<br/>:5432 → remote:5432"]
        Conn2 --> Fwd3["goroutine: Dynamic Forward<br/>:1080 (SOCKS)"]

        Monitor --> |切断検知| Reconnect["Backoff → 再接続"]
        Metrics --> |イベント| Broker
        Broker --> |通知| Client2
    end
```

- 各クライアント接続は独立した goroutine で処理
- Event Broker がイベントを集約し、サブスクライブ中のクライアントに配信
- `context.Context` でキャンセルを伝播し、グレースフルシャットダウンを実現
- Core Layer / Infra Layer の並行処理モデルは v1 から変更なし

## ファイルレイアウト

```mermaid
graph LR
    subgraph "~/.config/moleport/"
        Config["config.yaml<br/>ユーザー設定"]
        State["state.yaml<br/>セッション状態"]
        Log["moleport.log<br/>ログファイル"]
        PID["moleport.pid<br/>PID ファイル"]
        Sock["moleport.sock<br/>Unix ソケット"]
    end

    subgraph "~/.ssh/"
        SSHConfig["config"]
        SSHConfigD["config.d/"]
    end

    SSHConfig --> |読み取り専用| Daemon["デーモン"]
    SSHConfigD --> |読み取り専用| Daemon
    Config --> |読み書き| Daemon
    State --> |読み書き| Daemon
    Daemon --> |書き込み| Log
    Daemon --> |管理| PID
    Daemon --> |Listen| Sock

    CLI["CLI"] --> |接続| Sock
    TUI["TUI"] --> |接続| Sock
```

## 改訂履歴

| 版 | 日付 | 変更内容 | 変更理由 |
|---|------|---------|---------|
| 1.0 | 2026-02-10 | 初版作成 | — |
| 1.1 | 2026-02-10 | TUI を Atomic Design に再設計、図を Mermaid に変更 | ユーザー要望 |
| 2.0 | 2026-02-11 | デーモン + クライアントアーキテクチャに全面改訂。IPC Layer / CLI Layer 追加。JSON-RPC 2.0 over Unix Socket 採用 | デーモン化対応 |
| 2.1 | 2026-02-11 | クレデンシャルコールバック通信フロー追加、credential.request/response メソッド追加、pending_auth 状態フロー追加、PasswordInput コンポーネント追加 | #11 クレデンシャル入力機能追加 |
