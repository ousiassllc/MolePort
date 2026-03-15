# Changelog

## [v1.0.0] - 2026-03-15

### ⚠️ Breaking Changes / 破壊的変更
- First stable release — API and CLI interface are now considered stable / 初の安定版リリース — API と CLI インターフェースを安定版とする

### ✨ New Features / 新機能
- Add i18n core package with English/Japanese locale support / i18n コアパッケージを追加し英語・日本語ロケールに対応 (#79)
- Add language selection page (LangPage) to TUI / TUI に言語選択ページを追加 (#79)
- Internationalize all CLI and TUI text / CLI・TUI の全テキストを i18n 対応 (#79)
- Add TUI color theme system with theme selection page / TUI カラーテーマシステムとテーマ選択ページを追加 (#79)
- Add latest version check via `version.check` IPC handler / 最新バージョンチェック機能を追加 (#45)
- Add version notification dialog on TUI startup / TUI 起動時の最新バージョン通知ダイアログを追加 (#45)
- Add `moleport update` self-update command / セルフアップデートコマンドを追加 (#59)
- Add `version` CLI command with latest version check / `version` コマンドに最新バージョンチェックを追加 (#45)
- Add version mismatch detection and restart prompt on TUI startup / TUI 起動時のバージョン不一致検出と再起動提案を追加
- Add GoReleaser config and GitHub Actions release workflow / GoReleaser 設定と GitHub Actions リリースワークフローを追加 (#59)
- Add help overlay modal in TUI / TUI でヘルプをオーバーレイモーダル表示に変更
- Add version display on `v` key press in TUI / TUI で v キー押下時にバージョン表示
- Add `daemon.status` version info in response / `daemon.status` レスポンスにバージョン情報を追加
- Add wizard placeholder auto-fill and setuppanel subpackage / ウィザード placeholder 自動入力と setuppanel サブパッケージ化 (#66)
- Display rule name in TUI forward rows / TUI のフォワード行にルール名を表示 (#64)
- Add RemoteForward bind address configuration (default `127.0.0.1`) / RemoteForward バインドアドレスを設定可能に (#75)
- Support multiple IdentityFiles per SSH host / IdentityFile 複数対応 (#75)
- Add daemon startup warnings via `DaemonStatusResult` / daemon 起動時警告を通知、saveState にリトライ追加
- Add `EnsureDaemon()` auto-start logic / デーモン自動起動ロジックを追加
- Add `.env.example` template / `.env.example` テンプレートを追加

### 🐛 Bug Fixes / バグ修正
- Fix `Close()` to kill entire process group / `Close()` でプロセスグループ全体を Kill (#57)
- Fix forward bridge with half-close propagation and buffer pooling / forward bridge に half-close 伝播とバッファプーリングを追加
- Fix race conditions and concurrency safety / レースコンディションと並行処理の安全性を改善
- Fix IPC validation and error handling / IPC バリデーション強化・エラーハンドリング改善
- Fix daemon restart suppressing in-flight IPC errors / デーモン再起動中の IPC エラーログ抑制
- Fix repeated "not connected" error during daemon restart / 再起動中の "not connected" エラー繰り返し表示を修正
- Fix `context.Background()` to propagate parent context / `context.Background()` を親コンテキスト伝播に変更
- Fix config and statuscmd output i18n consistency / config と statuscmd の出力を i18n 対応に統一
- Fix Makefile VERSION to auto-detect from git tags / Makefile の VERSION を git タグから自動取得に変更
- Add linterly check to CI / CI に linterly チェックを追加
- Add CLI error context and unify `defer client.Close()` pattern / CLI エラーメッセージにコンテキストを付与し defer パターンを統一
- Fix unchecked `Close()` errors with log output / `Close()` エラーの未チェック箇所にログ出力を追加
- Add ProxyCommand alternative hint for unsupported ProxyJump / ProxyJump 未対応警告に ProxyCommand 代替案内を追加
- Log i18n template errors via `slog.Debug` / i18n テンプレートエラーを slog.Debug でログ出力

### ♻️ Refactoring / リファクタリング
- Extract socks5 to `core/socks5` subpackage / socks5 を `core/socks5` サブパッケージに分離 (#73)
- Extract proxycommand to `infra/proxycommand` subpackage / proxycommand を `infra/proxycommand` サブパッケージに分離
- Extract config handler to `handler/config` subpackage / config ハンドラを `handler/config` サブパッケージに分離
- Extract forward test mocks to `forwardtest` package / forward テストモックを `forwardtest` パッケージに分離
- Split `sshconn.go` forwarding/KeepAlive into `sshconn_forward.go` / sshconn.go からフォワーディング・KeepAlive を分離
- Unify IPC client setup with `DaemonCall` helper / CLI の IPC クライアントセットアップを DaemonCall ヘルパーに統一
- Replace error string matching with type-based classification / エラー文字列マッチングを型ベースのエラー分類に改善
- Extract magic numbers and method name strings to named constants / マジックナンバーとメソッド名文字列を名前付き定数に抽出
- Separate `MainModel` state fields into `dialogState`/`pageState` / MainModel の状態フィールドを分離
- Extract `handleForwardAdd` rollback logic to standalone function / ロールバックロジックを独立関数に抽出
- Centralize port range validation in core package / ポート範囲検証を core パッケージに共通化
- Centralize credential timeout to `core.CredentialTimeout` / クレデンシャルタイムアウトを共通化
- Remove CLI-to-infra direct dependency (layer violation) / cli から infra への直接依存を除去
- Unify `panelInnerSize` as public function and rename `render*` to `view*` prefix / panelInnerSize を公開関数に統一し render* を view* に統一

### 🧪 Tests / テスト
- Add CLI test coverage for all subcommands / CLI 全サブコマンドのテストカバレッジを向上
- Add infra package SSH server integration tests / infra パッケージに統合テストを追加
- Add TUI component tests (keys, molecules, organisms, pages) / TUI コンポーネントテストを追加
- Add format package unit tests / format パッケージのユニットテストを追加
- Add i18n package tests / i18n パッケージテストを追加
- Add update/checker and update/updater tests / update パッケージのテストを追加
- Add daemon state and ensure tests / daemon state・ensure テストを追加

### 📝 Documentation / ドキュメント
- Add SSH compatibility improvement design docs / SSH 互換性改善の設計ドキュメントを追記 (#75)
- Add self-update feature specs (`moleport update`) / セルフアップデート機能の仕様を追記 (#59)
- Add latest version check feature specs / 最新バージョンチェック機能の仕様を追加 (#45)
- Add TUI theme system design docs / TUI テーマシステム設計ドキュメントを追記
- Add i18n design docs / 多言語対応の設計ドキュメントを追加
- Update README with `update` command and TUI keybindings / README に update コマンドと TUI キーバインドを追加
- Sync architecture directory structure with implementation / アーキテクチャ設計のディレクトリ構成を実装と同期
- Fix multiple documentation drifts / ドキュメント乖離を修正 (#63, #70, #71)

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
