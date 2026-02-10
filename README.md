# MolePort

SSH ポートフォワーディングを視覚的に管理する TUI アプリケーション。

`~/.ssh/config` からホスト情報を読み込み、ポート転送の設定・接続・切断をダッシュボード上で直感的に操作できます。

```
┌─────────────────────────────────────────────────────┐
│  SSH Hosts                                          │
│  ● prod-server  192.168.1.10  user@22    2 active   │
│  ○ staging      10.0.0.5      deploy@22  0 active   │
├─────────────────────────────────────────────────────┤
│  Port Forwards: prod-server                         │
│  L :8080 → localhost:80    ● Active  2h 15m  ↑1.2MB │
│  L :5432 → localhost:5432  ● Active  45m     ↑52KB  │
├─────────────────────────────────────────────────────┤
│  > _                                                │
└─────────────────────────────────────────────────────┘
```

## 機能

- **SSH config 連携** — `~/.ssh/config`（Include 対応）からホストを自動読み込み
- **3種類の転送** — Local (-L) / Remote (-R) / Dynamic SOCKS5 (-D)
- **リアルタイム監視** — 接続状態、稼働時間、転送データ量を表示
- **自動再接続** — 指数バックオフで自動リトライ
- **セッション復元** — 前回のアクティブ転送を起動時に自動復元
- **対話型コマンド** — ステップバイステップで転送ルールを追加

## 必要環境

- Go 1.23+
- Linux / macOS
- ターミナル（256色以上推奨）

## インストール

```bash
# ソースからビルド
git clone https://github.com/ousiassllc/MolePort.git
cd MolePort
make build

# 実行
make run

# または直接
./bin/moleport
```

## 使い方

### キーバインド

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

### コマンド

| コマンド | 短縮 | 説明 |
|---------|------|------|
| `add` | `a` | 転送ルールを追加 |
| `delete` | `rm` | 転送ルールを削除 |
| `connect` | `c` | 停止中の転送を接続 |
| `disconnect` | `dc` | アクティブな転送を切断 |
| `list` | `ls` | 全ホスト・全ルールを表示 |
| `status` | `st` | 接続状態のサマリー |
| `config` | `cfg` | 設定を変更 |
| `reload` | — | SSH config を再読み込み |
| `help` | `?` | ヘルプ表示 |
| `quit` | `q` | 終了 |

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

## アーキテクチャ

3層構成 + Atomic Design:

```
TUI Layer (Bubble Tea + Lip Gloss)
  Pages → Organisms → Molecules → Atoms

Core Layer
  SSHManager / ForwardManager / ConfigManager

Infrastructure Layer
  SSHConnection / SSHConfigParser / YAMLStore
```

詳細は [docs/](docs/) を参照。

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
