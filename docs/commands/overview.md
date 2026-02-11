# MolePort コマンド仕様

## 概要

MolePort はサブコマンド形式の CLI で操作する。

```
moleport <subcommand> [options] [arguments]
```

## サブコマンド一覧

| サブコマンド | 引数 | 説明 |
|------------|------|------|
| `daemon start` | — | デーモンをバックグラウンドで起動 |
| `daemon stop` | `[--purge]` | デーモンを停止 |
| `daemon status` | — | デーモンの稼働状態を表示 |
| `connect` | `<host>` | SSH ホストに接続 |
| `disconnect` | `<host>` | SSH ホストを切断 |
| `add` | `--host, --local-port, ...` | 転送ルールをフラグ指定で追加 |
| `delete` | `<name>` | 転送ルールを削除 |
| `start` | `<name>` | 転送ルールのフォワーディングを開始 |
| `stop` | `<name> \| --all` | 転送ルールのフォワーディングを停止 |
| `list` | `[--host <host>] [--json]` | ホスト・転送ルールの一覧を表示 |
| `status` | `[name] [--json]` | 接続状態サマリー / セッション詳細を表示 |
| `config` | `[--json]` | 現在の設定を表示 |
| `reload` | — | SSH config を再読み込み |
| `tui` | — | TUI ダッシュボードを起動 |
| `help` | `[<subcommand>]` | ヘルプを表示 |
| `version` | — | バージョン情報を表示 |

## サブコマンド詳細

---

### daemon start

デーモンをバックグラウンドで起動する。

```
moleport daemon start
```

**動作**:
1. 既存デーモンの稼働確認（PID ファイル + ソケット疎通）
2. 未稼働なら自己フォークしてバックグラウンド化
3. PID ファイル・Unix ソケットを作成
4. config.yaml 読み込み、セッション復元、auto_connect 実行

**出力例**:

```
$ moleport daemon start
デーモンを起動しました (PID: 12345)

$ moleport daemon start
デーモンは既に稼働中です (PID: 12345)
```

---

### daemon stop

デーモンを停止する。

```
moleport daemon stop [--purge]
```

**フラグ**:

| フラグ | 説明 |
|--------|------|
| `--purge` | 状態ファイルを削除して停止 |

**動作**:
1. デーモンに shutdown リクエストを送信
2. 全接続をグレースフルに切断し、状態を保存
3. `--purge` 指定時は状態ファイルも削除

**出力例**:

```
$ moleport daemon stop
デーモンを停止しました

$ moleport daemon stop --purge
デーモンを停止しました（状態をクリア）

$ moleport daemon stop
デーモンは稼働していません
```

---

### daemon status

デーモンの稼働状態を表示する。

```
moleport daemon status
```

**出力例**:

```
$ moleport daemon status
MolePort Daemon:
  PID:        12345
  Uptime:     3h 30m
  Clients:    1 connected
  SSH:        2 connections
  Forwards:   3 active

$ moleport daemon status
デーモンは稼働していません
```

---

### connect

SSH ホストに接続する。auto_connect ルールのフォワーディングも自動的に開始される。

```
moleport connect <host>
```

**出力例**:

```
$ moleport connect prod-server
prod-server に接続しました
  ✓ prod-web (L :8080 → localhost:80) を開始しました
  ✓ prod-db (L :5432 → localhost:5432) を開始しました
```

**出力例（パスワード認証）**:

```
$ moleport connect password-server
Password: ********
password-server に接続しました
```

**出力例（パスフレーズ付き秘密鍵）**:

```
$ moleport connect prod-server
Enter passphrase for key '/home/user/.ssh/id_ed25519': ********
prod-server に接続しました
  ✓ prod-web (L :8080 → localhost:80) を開始しました
```

**出力例（keyboard-interactive / 2FA）**:

```
$ moleport connect 2fa-server
Password: ********
Verification code: 123456
2fa-server に接続しました
```

**出力例（認証キャンセル）**:

```
$ moleport connect password-server
Password: ^C
エラー: 認証がキャンセルされました
```

**エラー例**:

```
$ moleport connect unknown-host
エラー: ホスト 'unknown-host' が見つかりません

$ moleport connect prod-server
prod-server は既に接続済みです
```

---

### disconnect

SSH ホストを切断する。当該ホストの全フォワーディングも停止する。

```
moleport disconnect <host>
```

**出力例**:

```
$ moleport disconnect prod-server
prod-server を切断しました (2 forwards stopped)
```

---

### add

転送ルールをフラグ指定で追加する。

```
moleport add --host <host> --local-port <port> [options]
```

**フラグ**:

| フラグ | 必須 | デフォルト | 説明 |
|--------|------|-----------|------|
| `--host` | Yes | — | SSH ホスト名 |
| `--type` | No | `local` | 転送種別: `local`, `remote`, `dynamic` |
| `--local-port` | Yes | — | ローカルポート (1–65535) |
| `--remote-host` | No | `localhost` | リモートホスト |
| `--remote-port` | ※ | — | リモートポート (1–65535)。`local`/`remote` 転送で必須 |
| `--name` | No | 自動生成 | ルール名 |
| `--auto-connect` | No | `false` | 起動時に自動接続 |

**出力例**:

```
$ moleport add --host prod-server --local-port 8080 --remote-port 80
ルール 'prod-server-local-8080' を追加しました

$ moleport add --host prod-server --type dynamic --local-port 1080 --name socks
ルール 'socks' を追加しました
```

**バリデーション**:

| 条件 | エラーメッセージ |
|------|----------------|
| `--host` 未指定 | `--host フラグは必須です` |
| `--local-port` 未指定 | `--local-port フラグは必須です` |
| ポート番号が範囲外 | `ポート番号は 1〜65535 の範囲で入力してください` |
| `--type` が不正 | `--type は local, remote, dynamic のいずれかを指定してください` |
| `local`/`remote` で `--remote-port` 未指定 | `--remote-port フラグは local/remote 転送で必須です` |

---

### delete

転送ルールを削除する。

```
moleport delete <name>
```

**出力例**:

```
$ moleport delete prod-web
ルール 'prod-web' を削除しますか？ [y/N]: y
✓ ルール 'prod-web' を削除しました

$ moleport delete unknown
エラー: ルール 'unknown' が見つかりません
```

---

### start

転送ルールのフォワーディングを開始する。SSH 未接続の場合は自動的に接続する。

```
moleport start <name>
```

**出力例**:

```
$ moleport start prod-web
✓ prod-web (L :8080 → localhost:80) を開始しました
```

---

### stop

転送ルールのフォワーディングを停止する。SSH 接続は維持する。

```
moleport stop <name>
moleport stop --all
```

**フラグ**:

| フラグ | 説明 |
|--------|------|
| `--all` | 全フォワーディングを一括停止 |

**出力例**:

```
$ moleport stop prod-web
prod-web を停止しました

$ moleport stop --all
全フォワーディングを停止しました (3 件)
```

---

### list

全ホストと転送ルールの一覧を表示する。

```
moleport list [--host <host>] [--json]
```

**フラグ**:

| フラグ | 説明 |
|--------|------|
| `--host <host>` | 特定ホストのルールのみ表示 |
| `--json` | JSON 形式で出力 |

**出力例**:

```
$ moleport list
SSH Hosts (3 hosts, 1 connected):

● prod-server (192.168.1.10:22, user)
  L  :8080  ───►  localhost:80     ⬤ Active   2h 15m  ↑1.2MB ↓340KB
  L  :5432  ───►  localhost:5432   ⬤ Active   45m     ↑52KB  ↓128KB

⏳ 2fa-server (10.0.0.20:22, admin)  [Pending Auth]
  L  :3000  ───►  localhost:3000   ○ Stopped (auto_connect)

○ staging (10.0.0.5:22, deploy)
  D  :1080                         ○ Stopped

○ dev-db (172.16.0.3:5432, admin)
  (転送ルールなし)

$ moleport list --host prod-server
● prod-server (192.168.1.10:22, user)
  L  :8080  ───►  localhost:80     ⬤ Active   2h 15m  ↑1.2MB ↓340KB
  L  :5432  ───►  localhost:5432   ⬤ Active   45m     ↑52KB  ↓128KB
```

---

### status

接続状態を表示する。引数なしで全体サマリー、ルール名を指定するとセッション詳細を表示する。

```
moleport status [name] [--json]
```

**フラグ**:

| フラグ | 説明 |
|--------|------|
| `--json` | JSON 形式で出力 |

**出力例（サマリー）**:

```
$ moleport status
MolePort Status:
  Daemon:    Running (PID: 12345, uptime: 3h 30m)
  Hosts:     3 total, 1 connected, 1 pending auth
  Forwards:  3 total, 2 active, 1 stopped
  Traffic:   sent 1.3MB, recv 468.0KB

  ⏳ Pending Auth:
    2fa-server — 認証情報が必要です (moleport connect 2fa-server で接続)
```

**出力例（セッション詳細）**:

```
$ moleport status prod-web
Session: prod-web
  Host:           prod-server
  Type:           local
  Local Port:     8080
  Remote:         localhost:80
  Status:         active
  Connected At:   2026-02-11 10:30:00
  Bytes Sent:     1.2MB
  Bytes Received: 340.0KB
```

---

### config

現在の設定を表示する。

```
moleport config [--json]
```

**フラグ**:

| フラグ | 説明 |
|--------|------|
| `--json` | JSON 形式で出力 |

**出力例**:

```
$ moleport config
MolePort Config:
  SSH Config:     ~/.ssh/config
  Reconnect:
    Enabled:      true
    Max Retries:  10
    Initial Delay: 1s
    Max Delay:    60s
  Session:
    Auto Restore: true
  Log:
    Level:        info
    File:         ~/.config/moleport/moleport.log
```

---

### reload

SSH config を再読み込みし、ホスト一覧を更新する。

```
moleport reload
```

**出力例**:

```
$ moleport reload
SSH config を再読み込みしました
  4 ホスト読み込み（新規: 1, 削除: 0）
  + new-server が追加されました
```

---

### tui

デーモンに接続し、TUI ダッシュボードを起動する。

```
moleport tui
```

**動作**:
1. デーモンに IPC 接続
2. イベントサブスクリプション開始
3. ダッシュボード表示
4. TUI 終了時: サブスクリプション解除、IPC 切断（デーモンは継続）

**エラー**:

```
$ moleport tui
エラー: デーモンが稼働していません。moleport daemon start で起動してください。
```

---

### help

ヘルプを表示する。

```
moleport help [<subcommand>]
```

**出力例**:

```
$ moleport help
MolePort - SSH ポートフォワーディングマネージャ

Usage:
  moleport <command> [arguments]

Commands:
  daemon start       デーモンをバックグラウンドで起動
  daemon stop [--purge]  デーモンを停止（--purge: 状態クリア）
  daemon status      デーモンの稼働状態を表示
  connect <host>     SSH ホストに接続
  disconnect <host>  SSH ホストを切断
  add [flags]        転送ルールを追加
  delete <name>      転送ルールを削除
  start <name>       フォワーディングを開始
  stop <name> / --all  フォワーディングを停止（--all: 全停止）
  list [--json]      ホスト・転送ルールの一覧
  status [name]      接続状態のサマリー
  config [--json]    設定を表示
  reload             SSH config を再読み込み
  tui                TUI ダッシュボードを起動
  help               このヘルプを表示
  version            バージョン情報を表示

Global Flags:
  --config-dir <path>  設定ディレクトリのパス
```

---

### version

バージョン情報を表示する。

```
moleport version
```

**出力例**:

```
$ moleport version
MolePort 0.1.0 (go1.23.0, linux/amd64)
```

## TUI 内コマンド

TUI ダッシュボード内のコマンド入力欄で使用できるコマンド。
内部的に IPC 経由でデーモンに送信される。

| コマンド | 短縮形 | 説明 |
|---------|--------|------|
| `add` | `a` | 新しいポートフォワーディングルールを追加する |
| `delete` | `rm` | ポートフォワーディングルールを削除する |
| `connect` | `c` | SSH ホストに接続する |
| `disconnect` | `dc` | SSH ホストを切断する |
| `start` | — | 転送ルールのフォワーディングを開始する |
| `stop` | — | 転送ルールのフォワーディングを停止する |
| `list` | `ls` | 全ホスト・全転送ルールの一覧を表示する |
| `status` | `st` | 接続状態のサマリーを表示する |
| `config` | `cfg` | 設定を表示する |
| `reload` | — | SSH config を再読み込みする |
| `help` | `?` | コマンドヘルプを表示する |
| `quit` | `q` | TUI を終了する（デーモンは継続） |

## TUI キーバインド

| キー | コンテキスト | 動作 |
|------|-----------|------|
| `↑` / `k` | ホスト一覧 / 転送一覧 | 上の項目を選択 |
| `↓` / `j` | ホスト一覧 / 転送一覧 | 下の項目を選択 |
| `Enter` | ホスト一覧 | 選択中のホストに SSH 接続 |
| `Enter` | 転送一覧 | 選択中の転送をトグル（開始/停止） |
| `Tab` | 全体 | ペイン間のフォーカス移動 |
| `d` | 転送一覧 | 選択中の転送を停止 |
| `x` | 転送一覧 | 選択中の転送を削除（確認あり） |
| `?` | 全体 | ヘルプ表示 |
| `/` | 全体 | コマンド入力欄にフォーカス |
| `Esc` | コマンド入力 / パスワード入力 | 入力をキャンセル・フォーカス解除 |
| `Ctrl+C` | 全体 | TUI を終了（デーモンは継続） |

## TUI のクレデンシャル入力ダイアログ

SSH 接続時にクレデンシャルが必要な場合、TUI 上にパスワード入力ダイアログを表示する。

```
┌─────────────────────────────────────────────────────────────────┐
│  SSH Authentication: prod-server                                │
│                                                                 │
│  Enter passphrase for key '/home/user/.ssh/id_ed25519':         │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ ●●●●●●●●                                               │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
│  [Enter] Submit  [Esc] Cancel                                   │
└─────────────────────────────────────────────────────────────────┘
```

- `echo: false` の場合は入力を `●` でマスク表示
- `echo: true` の場合（OTP 等）は入力をそのまま表示
- keyboard-interactive の複数チャレンジは順次ダイアログを表示

## デーモン未稼働時の動作

デーモンが稼働していない状態で CLI コマンドを実行した場合、統一されたエラーメッセージを表示する。

```
$ moleport connect prod-server
エラー: デーモンが稼働していません。moleport daemon start で起動してください。
```

`daemon start` と `help` と `version` 以外の全サブコマンドでこのエラーを返す。

## 改訂履歴

| 版 | 日付 | 変更内容 | 変更理由 |
|---|------|---------|---------|
| 1.0 | 2026-02-10 | 初版作成 | — |
| 2.0 | 2026-02-11 | TUI 内コマンドから CLI サブコマンド体系に全面改訂。daemon/connect/disconnect/add/delete/start/stop/list/status/config/reload/tui/help/version を定義 | デーモン化対応 |
| 2.1 | 2026-02-11 | connect コマンドにパスワード/パスフレーズ/KI 認証の出力例追加、status/list に pending_auth 表示追加、TUI クレデンシャル入力ダイアログ仕様追加 | #11 クレデンシャル入力機能追加 |
