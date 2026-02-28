# Changelog

## [v0.2.0] - 2026-02-28

### ✨ New Features / 新機能
- Auto-start `auto_connect` forward rules on daemon startup / daemon起動時にauto_connectフォワードルールを自動開始 (#32)
- TUI visual improvements: rounded borders, inline titles, status bar, dialog borders / TUIビジュアル改善（Rounded Border・ステータスバー・ダイアログボーダー） (#30)
- Per-host reconnect policy runtime integration / ホスト別再接続ポリシーのランタイム統合 (#29)
- Forward restoration after SSH reconnect / SSH再接続後のフォワード復元 (#29)
- Configurable KeepAlive interval / KeepAlive間隔を設定可能に (#29)
- Data model extensions: ReconnectConfig, HostConfig, ForwardEvent types / データモデル拡張 (#29)

### 🐛 Bug Fixes / バグ修正
- Fix status bar background color issues / ステータスバー背景色の問題を修正 (#30)
- Delete state.yaml on daemon kill to prevent stale session restore / daemon kill時にstate.yamlを削除して古いセッション復元を防止 (#30)
- Fix KeepAliveInterval test data race / KeepAliveIntervalテストのデータレースを修正 (#29)
- Add jitter (0-10%) to reconnect backoff / 再接続バックオフにジッターを追加 (#29)

### 📝 Documentation / ドキュメント
- Add auto_connect supplementary notes to UC-9 / UC-9にauto_connect補足を追加 (#32)
- Add TUI visual improvement specs / TUIビジュアル改善仕様を追記 (#30)
- Update architecture, component, data model, IPC, and requirements docs for reconnect features / 再接続関連の設計ドキュメントを全面更新 (#29)
- Fix documentation drift in architecture, component, and data model docs / 設計ドキュメントの乖離を修正 (#28)
- Add daemon kill command spec / daemon killコマンド仕様を追加 (#28)

## [v0.1.0] - 2026-02-26

Initial release of MolePort / MolePort 初回リリース

### ✨ New Features / 新機能
- SSH port forwarding manager with daemon+client architecture / daemon+client アーキテクチャの SSH ポートフォワーディングマネージャ (#1, #4)
- Local (-L), Remote (-R), and Dynamic SOCKS5 (-D) forwarding / 3種類の転送に対応 (#1)
- TUI dashboard with real-time monitoring / リアルタイム監視付き TUI ダッシュボード (#1)
- Auto-reconnect with exponential backoff / 指数バックオフによる自動再接続 (#1)
- Session restore on startup / 起動時のセッション復元 (#1)
- SSH credential input (password, passphrase, keyboard-interactive) / SSH クレデンシャル入力対応 (#12)
- ProxyCommand support for Tailscale SSH / ProxyCommand による Tailscale SSH 接続サポート (#24)
- StrictHostKeyChecking support / StrictHostKeyChecking のホスト鍵検証スキップ対応 (#24)
- `daemon kill` command for force-terminating unresponsive daemons / 応答しないデーモンの強制終了コマンド (#24)
- Linterly integration for code line count enforcement / Linterly によるコード行数制限の導入 (#18)

### 🐛 Bug Fixes / バグ修正
- Fix SSH handshake hang and IPC client close timeout / SSH ハンドシェイクハングと IPC Client のタイムアウトを修正 (#7)
- Fix forward start failure log display and stale rule cleanup / フォワード開始失敗時のログ表示とルール残存を修正 (#16)
- Fix TUI duplicate log messages / TUI ログメッセージの二重表示を修正 (#21)
- Fix credential callback missing on forward.start from TUI / TUI からの forward.start で認証コールバックが欠落する問題を修正 (#22)
- Fix Tailscale SSH connection issues with ProxyCommand / ProxyCommand サポートによる Tailscale SSH 接続問題を修正 (#24)
- Various code review fixes (errcheck, gosec, staticcheck) / コードレビュー指摘事項の修正 (#6)

### 🔧 Improvements / 改善
- Split infra package into yamlstore/sshconfig subpackages / infra パッケージを yamlstore/sshconfig サブパッケージに分割 (#24)
- Split core/ipc/tui packages into focused subpackages / core/ipc/tui パッケージをサブパッケージに分割 (#18)

### 📝 Documentation / ドキュメント
- Full design documentation (architecture, requirements, component design, IPC API spec) / 設計ドキュメント一式を作成
- Fix documentation drift from implementation / ドキュメント乖離の修正 (#9, #14)
- English README with Mermaid diagrams / README 英語版の作成と Mermaid ダイアグラム化
