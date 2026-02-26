# Changelog

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
