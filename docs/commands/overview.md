# MolePort コマンド仕様

## 概要

MolePort はサブコマンド形式の CLI で操作する。

```
moleport <subcommand> [options] [arguments]
```

サブコマンドを省略して `moleport` のみで実行すると、TUI ダッシュボードが起動する（`moleport tui` と同等）。

### 出力言語

以下の出力例は表示言語が日本語（`ja`）の場合の実際の出力である。英語（`en`）では同じ構造で文言のみ英語になる（文言は `internal/i18n/locales/{ja,en}.yaml` の `cli.*`）。エラーは stderr に `エラー: <メッセージ>`（`en` では `Error: <message>`）の形式で出力され、終了コードは 1 になる。デーモンが返したエラーは `rpc error: code=<コード>, message=<メッセージ>` の形式でそのまま表示される（エラーコードは [IPC API 仕様](../api/ipc.md) を参照）。

### グローバルフラグ

| フラグ | 説明 |
|--------|------|
| `--config-dir <path>` | 設定ディレクトリのパス。ソケット・PID ファイル・`config.yaml`・`state.yaml` はこのディレクトリに置かれる |

設定ディレクトリは次の優先順位で解決する（`internal/cli/root.go` の `ResolveConfigDir`。`moleport-gui` も同じ）。

1. `--config-dir <path>`
2. 環境変数 `MOLEPORT_CONFIG_DIR`
3. 環境変数 `XDG_CONFIG_HOME` が設定されている場合は `$XDG_CONFIG_HOME/moleport`
4. `~/.config/moleport`（既定値）

以降のドキュメントに現れる `~/.config/moleport/` は、上記の既定値で解決された場合のパスを指す。

## サブコマンド一覧

| サブコマンド | 引数 | 説明 |
|------------|------|------|
| `daemon start` | — | デーモンをバックグラウンドで起動 |
| `daemon stop` | `[--purge]` | デーモンを停止 |
| `daemon status` | — | デーモンの稼働状態を表示 |
| `daemon kill` | — | デーモンを強制終了（応答しない場合） |
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
| `update` | `[--check]` | 最新バージョンに自動アップデート |
| `help` | — | ヘルプを表示（サブコマンド別ヘルプは未実装。引数は無視される） |
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
MolePort デーモン:
  バージョン: 0.1.0
  PID:        12345
  稼働時間:   3h 30m
  クライアント: 1 接続中
  SSH:        2 接続
  フォワード:  3 アクティブ
  警告:       update_check.interval 30m0s is invalid (minimum 1h0m0s); using 1h0m0s

$ moleport daemon status
デーモンは稼働していません
```

`警告:` 行はデーモンが起動時などに検出した警告ごとに 1 行ずつ、フォワード行の後に表示される（警告がなければ表示されない）。メッセージは英語のまま表示される。主な内容: `config.yaml` の読み込み失敗（デフォルト値で動作。元のファイルは設定保存時に `config.yaml.bak` へ退避される）、`update_check.interval` の補正、設定ディレクトリ・`config.yaml`・`state.yaml`・SSH config のパーミッションが group/other に開放されている場合（Unix のみ）、フォワードルール・SSH ホストの読み込み失敗。

---

### daemon kill

応答しないデーモンを強制終了する。IPC 経由の停止（`daemon stop`）が応答しない場合に使用する。

```
moleport daemon kill
```

**動作**:
1. PID ファイルからデーモンの PID を読み取る
2. SIGKILL でプロセスを強制終了する
3. PID ファイルを削除する
4. `state.yaml` を削除する（強制終了では状態保存が走らないため、残すと次回起動時に古いセッションが自動復元されてしまうのを防ぐ）

**注意**:
- IPC 通信は行わず、直接プロセスを kill する
- グレースフルな切断や状態保存は行われない。通常は `daemon stop` を使用すること

**出力例**:

```
$ moleport daemon kill
デーモンを強制終了しました (PID: 12345)

$ moleport daemon kill
デーモンは稼働していません
```

---

### connect

SSH ホストに接続する。auto_connect ルールのフォワーディングはデーモンが自動的に開始する（開始したルールの一覧は出力されない。`moleport list` で確認できる）。接続済みのホストに対しては SSH 接続を再確立せず成功を返すが、未開始（`stopped` / `error`）の auto_connect ルールは開始される（`moleport stop` で止めたルールも再開される）。

```
moleport connect <host>
```

**出力例**:

```
$ moleport connect prod-server
prod-server に接続しました
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
エラー: connect failed: rpc error: code=1009, message=failed to connect to password-server: ... credential cancelled
```

**エラー例**:

```
$ moleport connect unknown-host
エラー: connect failed: rpc error: code=1001, message=host "unknown-host" not found
```

接続済みのホストに対して再度実行してもエラーにならず、そのまま成功する（SSH 接続は再確立されない。ただし未開始の auto_connect ルールは上記のとおり開始される）。

---

### disconnect

SSH ホストを切断する。当該ホストの全フォワーディングも停止する。

```
moleport disconnect <host>
```

**出力例**:

```
$ moleport disconnect prod-server
prod-server を切断しました
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
| `--name` | No | 自動生成（`forward-<N>`） | ルール名。省略時は追加時の出力でルール名を確認する |
| `--remote-bind-addr` | No | `127.0.0.1` | リモート側のバインドアドレス（`remote` 転送でのみ有効。OpenSSH の `RemoteForward` の bind_address 相当）。省略時は `127.0.0.1` |
| `--auto-connect` | No | `false` | 起動時に自動接続 |

**出力例**:

```
$ moleport add --host prod-server --local-port 8080 --remote-port 80
ルール 'forward-1' を追加しました

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
prod-web を削除しました

$ moleport delete unknown
エラー: rpc error: code=1004, message=rule "unknown" not found
```

---

### start

転送ルールのフォワーディングを開始する。SSH 未接続の場合は自動的に接続する。ホストが再接続待機中（`Reconnecting`）の場合は、自動再接続を打ち切らないため接続せず、`NotConnected`（エラーコード 1003）で失敗する。再接続の完了後に再実行する。

```
moleport start <name>
```

**出力例**:

```
$ moleport start prod-web
prod-web を開始しました
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
SSH ホスト (3 件, 1 件接続中):

● prod-server (192.168.1.10:22, user)
  L  :8080  ->  localhost:80
  L  :5432  ->  localhost:5432

◎ 2fa-server (10.0.0.20:22, admin)
  L  :3000  ->  localhost:3000

○ staging (10.0.0.5:22, deploy)
  D  :1080

○ dev-db (172.16.0.3:5432, admin)
  (転送ルールなし)

$ moleport list --host prod-server
● prod-server (192.168.1.10:22, user)
  L  :8080  ->  localhost:80
  L  :5432  ->  localhost:5432
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
MolePort ステータス:
  デーモン:    稼働中 (PID: 12345, 稼働時間: 3h 30m)
  ホスト:      3 件, 1 件接続中, 1 件認証待ち
  フォワード:  3 件, 2 件アクティブ, 1 件停止
  トラフィック: 送信 1.3MB, 受信 468.0KB
  警告:       update_check.interval 30m0s is invalid (minimum 1h0m0s); using 1h0m0s
```

サマリー出力の末尾に、`daemon status` と同じ警告をデーモンが検出した分だけ `警告:` 行として表示する（警告がなければ表示されない。セッション詳細では表示されない）。

**出力例（セッション詳細）**:

```
$ moleport status prod-web
セッション: prod-web
  ホスト:          prod-server
  タイプ:          local
  ローカルポート:  8080
  リモート:        localhost:80
  ステータス:      active
  接続日時:        2026-02-11 10:30:00
  送信バイト:      1.2MB
  受信バイト:      340.0KB
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
MolePort 設定:
  SSH Config:     ~/.ssh/config
  再接続:
    有効:          true
    最大リトライ:  10
    初期遅延:      1s
    最大遅延:      60s
  セッション:
    自動復元:      true
  ログ:
    レベル:        info
    ファイル:      ~/.config/moleport/moleport.log
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

**デーモン未稼働時**:

デーモンが未稼働の場合は自動的に起動する。

---

### help

ヘルプを表示する。サブコマンド別ヘルプ（`moleport help <subcommand>`）は提供しておらず、引数を渡しても無視して常に全体ヘルプを表示する。

```
moleport help
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
  daemon kill        デーモンを強制終了（応答しない場合）
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
  update [--check]   最新バージョンに自動アップデート
  help               このヘルプを表示
  version            バージョン情報を表示

Global Flags:
  --config-dir <path>  設定ディレクトリのパス
```

---

### update

最新バージョンのバイナリを GitHub Releases からダウンロードし、自動アップデートする。

```
moleport update [--check]
```

**フラグ**:

| フラグ | 説明 |
|--------|------|
| `--check` | アップデートの有無を確認するのみ（ダウンロード・置換は行わない） |

**動作**:
1. GitHub Releases API で最新バージョンを取得（デーモンのキャッシュは使わず、毎回 CLI が直接 API を呼ぶ）
2. 現在のバージョンと比較し、新しいバージョンがなければ終了
3. `--check` 指定時はここで終了
4. OS/アーキテクチャに対応するアセット（tar.gz）をダウンロード
5. チェックサムファイル（checksums.txt）をダウンロードし SHA-256 で検証
6. デーモンが稼働中の場合はグレースフルに停止し、PID ファイルのロック解放まで待機
7. 現在のバイナリをアトミックリネームで置換（一時ファイル → rename）
8. デーモンが稼働していた場合は再起動

**出力例**:

```
$ moleport update
最新バージョンを確認中...
v0.2.0 が利用可能です（現在: v0.1.0）
ダウンロード中: moleport_linux_amd64.tar.gz
チェックサム検証: OK
デーモンを停止中...
バイナリを更新中...
デーモンを再起動中...
✓ MolePort を v0.2.0 に更新しました
```

```
$ moleport update
最新バージョンを確認中...
現在のバージョン (v0.2.0) は最新です
```

```
$ moleport update --check
最新バージョンを確認中...
v0.3.0 が利用可能です（現在: v0.2.0）
  https://github.com/ousiassllc/moleport/releases/tag/v0.3.0
```

**エラー例**:

```
$ moleport update
最新バージョンを確認中...
エラー: バージョン情報の取得に失敗しました: connection refused

$ moleport update
最新バージョンを確認中...
v0.2.0 が利用可能です（現在: v0.1.0）
ダウンロード中: moleport_linux_amd64.tar.gz
エラー: チェックサム検証に失敗しました（ファイルが破損している可能性があります）

$ moleport update
エラー: dev ビルドではアップデートできません（リリースビルドをインストールしてください）
```

**注意**:
- バージョンが `dev` の場合はエラーとなる（開発ビルドはリリースバイナリで置換すべきでないため）
- バイナリの置換にはファイルへの書き込み権限が必要（権限不足時はエラーメッセージで `sudo` の使用を案内）
- ダウンロードまたは検証に失敗した場合、元のバイナリは変更されない
- デーモンの停止を確認できない場合（応答なし・タイムアウト）はバイナリを置換せずエラーとなり、`moleport daemon kill` を案内する。停止していないデーモンの再起動は行わない

---

### version

バージョン情報を表示する。

```
moleport version
```

**動作**:
1. ローカルのバージョン情報を常に表示する
2. デーモンが稼働している場合のみ、デーモンの `version.check` で最新バージョンを確認する（デーモンの自動起動は行わない）
3. 更新がある場合のみ、ローカルバージョンの後に更新の案内を表示する。最新の場合は何も表示しない
4. チェックに失敗した場合もローカルバージョンの表示は変わらず、エラーを stderr に出力する
5. `update_check.enabled` が `false` の場合・`dev` ビルドの場合は最新バージョンが得られないため、案内は表示されない

**出力例**:

```
$ moleport version
MolePort 0.1.0 (go1.23.0, linux/amd64)
```

更新がある場合（デーモン稼働中）:

```
$ moleport version
MolePort 0.1.0 (go1.23.0, linux/amd64)
アップデートがあります: v0.2.0 (現在: v0.1.0)
リリース: https://github.com/ousiassllc/moleport/releases/tag/v0.2.0
```

チェックに失敗した場合（stderr）:

```
$ moleport version
MolePort 0.1.0 (go1.23.0, linux/amd64)
バージョンチェックに失敗しました: rpc error: code=-32603, message=...
```

## TUI の操作方式

TUI はコマンド入力欄を持たず、キーバインドとウィザードで操作する。

### フォワード追加ウィザード

SetupPanel（SSH ホスト一覧）でホストを選択し、`Enter` キーを押すとフォワード追加ウィザードが開始される。
ウィザードは以下のステップで構成される:

1. **転送種別選択**: Local / Remote / Dynamic から選択
2. **ローカルポート**: ローカル側のポート番号を入力
3. **リモートホスト**: リモート側のホスト名を入力（Dynamic 転送ではスキップ）
4. **リモートポート**: リモート側のポート番号を入力（Dynamic 転送ではスキップ）
5. **ルール名**: 転送ルールの名前を入力（空欄で `<host>-<type>-<localport>` を採用）
6. **確認**: 入力内容を確認し、Enter で追加を実行

- `Esc` キーでウィザードをキャンセルし、ダッシュボードに戻る
- ウィザード完了後、転送ルールが追加され自動的にフォワーディングを開始する

### その他の TUI 操作

| 操作 | キー | 説明 |
|------|------|------|
| フォワード追加 | `Enter`（SetupPanel） | フォワード追加ウィザードを開始 |
| フォワード削除 | `x`（転送一覧） | 選択中の転送ルールを削除 |
| テーマ変更 | `t` | テーマ選択画面を表示 |
| 言語切替 | `l` | 言語切替画面を表示 |
| TUI 終了 | `q` / `Ctrl+C` | TUI を終了（デーモンは継続） |

## TUI キーバインド

| キー | コンテキスト | 動作 |
|------|-----------|------|
| `↑` / `k` | ホスト一覧 / 転送一覧 | 上の項目を選択 |
| `↓` / `j` | ホスト一覧 / 転送一覧 | 下の項目を選択 |
| `Enter` | ホスト一覧 | フォワード追加ウィザードを開始 |
| `Enter` | 転送一覧 | 選択中の転送をトグル（開始/停止） |
| `Tab` | 全体 | ペイン間のフォーカス移動 |
| `d` | 転送一覧 | 選択中の転送を停止 |
| `x` | 転送一覧 | 選択中の転送を削除（確認あり） |
| `q` | 全体 | TUI を終了（デーモンは継続） |
| `t` | 全体 | テーマ選択画面を表示 |
| `l` | 全体 | 言語切替画面を表示 |
| `v` | 全体 | バージョン情報を表示 |
| `?` | 全体 | ヘルプ表示 |
| `/` | 全体 | SetupPanel にフォーカス |
| `Esc` | ウィザード / パスワード入力 | 入力をキャンセル・フォーカス解除 |
| `Ctrl+C` | 全体 | TUI を終了（デーモンは継続） |

## TUI のクレデンシャル入力ダイアログ

SSH 接続時にクレデンシャルが必要な場合、TUI 上にパスワード入力ダイアログを表示する。

```
┌─────────────────────────────────────────────────────────────────┐
│  SSH Authentication: prod-server                                │
│                                                                 │
│  Enter passphrase for key '/home/user/.ssh/id_ed25519':         │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ ********                                                │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
│  [Enter] Submit  [Esc] Cancel                                   │
└─────────────────────────────────────────────────────────────────┘
```

- `echo: false` の場合は入力を `*` でマスク表示
- `echo: true` の場合（OTP 等）は入力をそのまま表示
- keyboard-interactive の複数チャレンジは順次ダイアログを表示

## デーモン未稼働時の動作

デーモンが稼働していない状態で CLI コマンドを実行した場合、デーモンを自動的に起動する。

```
$ moleport connect prod-server
デーモンを起動しました (PID: 12345)
prod-server に接続しました
```

`daemon start`、`daemon kill`、`tui`、`update`、`help`、`version` 以外の全サブコマンドでデーモンの自動起動を行う。

- `tui` はデーモン未稼働の場合も自動的にデーモンを起動する（TUI 内部で処理）
- `daemon kill` は IPC を使わず PID ファイルベースで動作するため、デーモン未稼働時は独自のメッセージ（「デーモンは稼働していません」）を表示する

## GUI（`moleport-gui`）稼働中の動作

GUI 稼働中は内蔵デーモンが `moleport.sock` / `moleport.pid` を保持しているため、CLI/TUI は headless デーモンと同様にこれへ接続する（自動起動は行われない）。

| コマンド | 挙動 |
|---------|------|
| `daemon start` | 「デーモンは既に稼働中です (PID: xxxx)」（PID は GUI プロセス） |
| `daemon stop` | GUI プロセスごとグレースフルに終了する（ウィンドウ・トレイも消える） |
| `daemon kill` | GUI プロセスを SIGKILL で強制終了する（Unix のみ） |
| `daemon status` | 稼働中。`version` は GUI のビルドバージョン |
| `update` | 稼働中のデーモン（= GUI）を停止してから CLI バイナリを置換し、**headless デーモンとして**再起動する（GUI は自動では再起動されない。`moleport-gui` は別途起動する。UC-24） |
| `tui` | 通常どおり接続する。バージョン不一致ダイアログで「はい」を選ぶと GUI が終了し headless デーモンが起動する（UC-17） |

その他のサブコマンドは変更なし。GUI 側の操作（トレイ・ウィンドウ）は CLI/TUI の操作にイベントとティッカーで追従する（UC-27）。

## 改訂履歴

| 版 | 日付 | 変更内容 | 変更理由 |
|---|------|---------|---------|
| 1.0 | 2026-02-10 | 初版作成 | — |
| 2.0 | 2026-02-11 | TUI 内コマンドから CLI サブコマンド体系に全面改訂。daemon/connect/disconnect/add/delete/start/stop/list/status/config/reload/tui/help/version を定義 | デーモン化対応 |
| 2.1 | 2026-02-11 | connect コマンドにパスワード/パスフレーズ/KI 認証の出力例追加、status/list に pending_auth 表示追加、TUI クレデンシャル入力ダイアログ仕様追加 | #11 クレデンシャル入力機能追加 |
| 2.2 | 2026-02-27 | `daemon kill` サブコマンドの仕様を追加 | #25 ドキュメント乖離修正 |
| 2.3 | 2026-02-27 | config コマンド出力例に KeepAlive 間隔と hosts セクション（ホスト別再接続ポリシー）を追加 | #27 自動再接続機能の改善・拡張 |
| 3.0 | 2026-03-01 | TUI 内コマンドをキーバインド+ウィザード方式に改訂、キーバインド表に q/t/l/v 追加、config/list/delete/disconnect/daemon status 出力例を実装に合わせて修正、デーモン未稼働時の自動起動に変更 | #40 ドキュメント乖離修正 |
| 3.1 | 2026-03-08 | `update` サブコマンド仕様追加、ヘルプ出力例・デーモン自動起動除外リストに追記 | #58 セルフアップデート機能 |
| 3.2 | 2026-10-15 | GUI 稼働中の動作セクションを追加（daemon start/stop/kill/status、update、tui の挙動） | #84 Wails 版 GUI アプリケーション |
| 3.3 | 2026-11-28 | 出力例を ja ロケールの実出力に修正（start/delete/list/status/config/daemon status、エラーは `rpc error` 形式）、出力言語の注記を追加、add に `--remote-bind-addr` を追記、help はサブコマンド別ヘルプ未実装と明記、グローバルフラグ節に設定ディレクトリ解決順（`XDG_CONFIG_HOME` 含む）を追記、daemon kill が `state.yaml` を削除する旨を追記、update はデーモンのキャッシュを使わない旨に修正、version の動作（最新時は無表示・失敗時は stderr）を明記、TUI パスワードのマスク文字を `*` に統一 | #122, #123, #124, #132, #134 ドキュメント乖離修正 |
| 3.4 | 2026-11-28 | `connect` が接続済みでも未開始の auto_connect ルールを開始することを明記、`daemon status` / `status` の出力例と説明に警告行（`警告:`）を追記 | #149, #151 ドキュメント乖離修正 |
| 3.5 | 2026-11-28 | `start` が再接続待機中（`Reconnecting`）のホストのルールに対して接続せず `NotConnected`（1003）で失敗することを明記 | #165 ドキュメント乖離修正 |
