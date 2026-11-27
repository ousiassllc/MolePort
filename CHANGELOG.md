# Changelog

## [Unreleased]

### ⚠️ Breaking Changes / 破壊的変更
- Windows GUI is no longer distributed as `moleport-gui_<version>_windows_amd64.zip`; use `moleport-gui_<version>_windows_amd64_installer.exe` instead / Windows 向け GUI の `moleport-gui_<version>_windows_amd64.zip` 配布を廃止し、`moleport-gui_<version>_windows_amd64_installer.exe`（インストーラー）に置き換え

### ✨ New Features / 新機能
- Add GUI Shell layer (`cmd/moleport-gui`, `internal/gui/shell`, `internal/gui/tray`; `gui` build tag) with built-in daemon startup, Router wiring to the Wails AssetServer, system tray wrapper and window-close handling / GUI の Shell 層（`gui` タグ）を追加。内蔵デーモン起動、Wails AssetServer への Router 接続、トレイのラッパー、ウィンドウ閉じる操作の分岐を実装 (#87)
- Add GUI dashboard (header, host/forward panels with HATEOAS actions, status bar), `DashboardService`, `EventBridge` (100ms coalescing, 1s live / 5s reconcile tickers, hidden-window suppression), `NotificationQueue` and `/fragments/{header,hosts,forwards,status,toasts}` / GUI ダッシュボード（ヘッダー・ホスト/転送パネル・ステータスバー）、`DashboardService`、`EventBridge`、`NotificationQueue`、各フラグメントを追加 (#88)
- Split i18n locale files per namespace (`<lang>.yaml` + `<lang>.gui.yaml`) and add `format.Duration` / 翻訳ファイルを名前空間別に分割し、`format.Duration` を追加 (#88)
- Add GUI operations: add (local/remote/dynamic form with server-side validation, placeholders and free-port suggestions on conflict), start, stop and delete forwards, connect/disconnect hosts (async, HATEOAS 409), and the credential dialog backed by `credential.Broker` (FIFO, cancel, 30s timeout, buffers zeroed, body-only) / GUI の操作系（転送の追加・開始・停止・削除、ホストの接続・切断、クレデンシャルダイアログと `credential.Broker`）を追加 (#89)
- Add GUI settings (language, theme `system`/`dark`/`light` × 5 accents with live preview and cancel, update check) saved to the new `gui:` section of `config.yaml` (backward compatible; `tui.theme` untouched), and the new-version badge that opens the release page via `POST /actions/update/open` (server-side URL prefix check). `VersionChecker` gains `SetEnabled` / `Cached` / GUI の設定画面（言語・テーマ・アップデートチェック、`config.yaml` の `gui:` セクション）と新バージョンバッジ・リリースページ起動を追加 (#90)
- Implement GUI process lifecycle and tray: headless-daemon takeover toast (old PID, `auto_restore` note), single `Daemon.Stop()` across tray quit / `daemon.shutdown` / OS signals, quit confirmation only for user-initiated quits with active forwards, first-hide OS notice (`gui.tray_notice_shown`), and the full tray menu (summary, show window, up to 20 forward toggles, stop all, quit) following state and language changes. `Window` / `Dialog` / `DaemonHost` interfaces keep the sequences testable without Wails; real-device verification pending / GUI のプロセスライフサイクルとトレイを実装: 引き継ぎトースト、終了の単一 Stop 化、終了確認、初回トレイ通知、UC-33 のフルメニュー（状態・言語追従）。Wails 非依存 interface でユニットテスト、実機検証待ち (#91)
- Add GUI distribution: `ci.yml` builds `-tags gui` on Linux / macOS / Windows and checks `templ generate` drift; `release.yml` builds `moleport-gui` on native runners after the GoReleaser job and attaches `moleport-gui_<version>_{linux_amd64,linux_arm64}.tar.gz`, `_darwin_universal.zip`, `_windows_amd64_installer.exe` (NSIS installer) and `checksums_gui.txt` to the same Release (existing `moleport` assets unchanged). README (en/ja) documents installation, the unsigned-build warnings (macOS / Windows SmartScreen) and the Linux tray requirement; `make gui-build` gains `GUI_TAGS` and embeds the version. Real-runner verification pending / GUI の配布を追加: CI の OS 別 `-tags gui` ビルドと Templ 生成物の差分検査、リリースでの GUI アセットと `checksums_gui.txt` の添付（既存 `moleport` アセットは不変。Windows は NSIS インストーラー）、README（ja/en）に導入手順・未署名警告の回避・Linux トレイ要件を追記、`make gui-build` に `GUI_TAGS` とバージョン埋め込みを追加（実ランナーでの検証待ち） (#92)
- Ship the Windows GUI as a per-user NSIS installer (`cmd/moleport-gui/build/windows/installer/project.nsi`, based on the Wails v2.16.0 template): no administrator rights, installs to `%LOCALAPPDATA%\Programs\MolePort`, HKCU uninstall entry, current-user shortcuts, in-place upgrade over the same install location, prompt to quit a running `moleport-gui.exe`, uninstall removes only installed files (settings under `.config\moleport` are kept). `release.yml` installs NSIS and uploads `moleport-gui_<version>_windows_amd64_installer.exe` (included in `checksums_gui.txt`); the `ci.yml` Windows GUI job also builds the installer and checks it exists. Still unsigned (SmartScreen: More info → Run anyway). Installing/upgrading on a real Windows machine is pending verification / Windows の GUI をユーザー単位の NSIS インストーラーで配布: 管理者権限不要、`%LOCALAPPDATA%\Programs\MolePort` にインストール、HKCU のアンインストール情報、現在のユーザー用ショートカット、同一インストール先への上書き更新、起動中の `moleport-gui.exe` の終了確認、アンインストールはインストールしたファイルのみ削除（`.config\moleport` の設定は残す）。`release.yml` が NSIS を導入して `moleport-gui_<version>_windows_amd64_installer.exe` を添付（`checksums_gui.txt` に含まれる）、`ci.yml` の Windows GUI ジョブでもインストーラー生成を検証。コード署名なし（SmartScreen は「詳細情報」→「実行」）。実機の Windows でのインストール・更新は実機検証待ち (#92)
- Document PoC procedures (R-1 to R-8) and fallback policies; real-device results are pending / PoC の検証手順（R-1〜R-8）と縮退方針を仕様書に記載（実機検証結果は待ち） (#87)
- Warn (log and `daemon.status` warnings; read-only, Unix only) when the config directory, `config.yaml`, `state.yaml` or the SSH config allow group/other access (NFR-09 / NFR-10) / 設定ディレクトリ・`config.yaml`・`state.yaml`・SSH config が group/other からアクセス可能な場合に警告（ログと `daemon.status` の warnings。読み取りのみ、Unix のみ）(#126, #130)

### 🐛 Bug Fixes / バグ修正
- Back up an unreadable `config.yaml` to `config.yaml.bak` before the first save instead of silently overwriting it with defaults, and report the load failure in the log and `daemon.status` warnings (shown by `moleport status` / `daemon status`) / 読み込めなかった `config.yaml` を、デフォルト値で無言で上書きせず最初の保存前に `config.yaml.bak` へ退避し、ログと `daemon.status` の warnings（`moleport status` / `daemon status` に表示）にエラーを出す (#101)
- Take the forward-rule snapshot inside the config lock in `PersistForwardRules` so concurrent add/delete operations no longer persist a stale list / `PersistForwardRules` のルール一覧取得を設定ロック内で行い、並行する追加・削除で古い一覧が保存されないよう修正 (#112)
- Clamp `update_check.interval` from `config.yaml` (<= 0 → 24h, < 1h → 1h) with a warning instead of panicking in `time.NewTicker` / `config.yaml` の `update_check.interval` が 0 以下・1h 未満でも panic せず補正して警告する (#115)
- Use a dedicated download HTTP client without the 5-second version-check timeout for `moleport update` / `moleport update` のダウンロードにバージョンチェック用の 5 秒タイムアウトを使わないよう修正 (#103)
- Apply `update_check.enabled` changes from IPC `config.update` to the running version checker / IPC `config.update` による `update_check.enabled` の変更を稼働中のバージョンチェッカーに反映 (#136)
- Add `object-src 'none'`, `base-uri 'none'`, `form-action 'self'` and `frame-ancestors 'none'` to the GUI Content-Security-Policy (these do not fall back to `default-src`) / GUI の Content-Security-Policy に `object-src 'none'`・`base-uri 'none'`・`form-action 'self'`・`frame-ancestors 'none'`（`default-src` にフォールバックしない指令）を追加 (#99)
- Fix the GUI credential dialog not updating its pending-request count, link form validation errors to their inputs via `aria-describedby`, and suppress duplicate generic error toasts / GUI のクレデンシャルダイアログの待機数が更新されない問題、フォームの検証エラーが入力欄に `aria-describedby` で関連付けられていない問題、汎用エラートーストが重複表示される問題を修正 (#95, #96, #98)
- Allow restarting forwards left in the Error state (reconnect or restore failure) with start / `ssh.connect` auto_connect instead of failing with `already active` / 再接続・復元の失敗で Error になった転送を、開始操作と `ssh.connect` の auto_connect で再開できるよう修正（`already active` で失敗していた）(#139)
- Stop the host's forwards on IPC `ssh.disconnect` (CLI/TUI), as documented, so no Active forward is left on a closed SSH client / IPC `ssh.disconnect`（CLI/TUI）が仕様どおり当該ホストの転送も停止するよう修正し、切断済みの SSH クライアントを掴んだ Active 転送が残らないように (#140)
- Fix an automatic reconnect succeeding after a manual connect made during the reconnect wait, which overwrote and leaked the healthy connection and later disconnected it by host name; the manual connection now takes precedence / 自動再接続の待機中に手動接続した場合、後続の再接続成功が接続を上書きしてリークし、ホスト名で健全な接続を切断し直す問題を修正（手動接続を優先）(#141)
- Make auto-generated `forward-<N>` rule names skip existing rules (e.g. after a daemon restart) and not consume an ID on validation errors / 自動生成名 `forward-<N>` が既存ルール（デーモン再起動後など）と衝突しないよう未使用の番号まで進め、検証エラーでは番号を消費しないよう修正 (#143)
- Deep-copy `Config` in `ConfigManager` so concurrent `config.update` / `config.get` / SSH reconnect lookups no longer crash the daemon with a concurrent map access, and a failed save leaves the cached config untouched / `ConfigManager` が `Config`（`hosts`・`forwards`）をディープコピーして扱うようにし、`config.update` と `config.get`・SSH 再接続設定の並行アクセスによるデーモンの異常終了と、保存失敗時にキャッシュだけが変わる不整合を修正 (#142)
- Emit `aria-invalid="true"` (instead of a value-less attribute) on invalid GUI inputs / GUI の検証エラー入力欄が値なしの `aria-invalid` ではなく `aria-invalid="true"` を出力するよう修正 (#137)
- Switch the GUI generic error toast text and close label to the new language right after the language is saved / GUI の言語を保存した直後から、汎用エラートーストの文言と閉じるボタンのラベルが新しい言語になるよう修正 (#138)
- Fix a forward start in progress (waiting for SSH connect / opening the listener) not being cancelled by `StopForward` / `DeleteRule` / `Close`, which left an orphaned forward; `StartForward` after `Close` now fails / 転送の開始処理中に停止・ルール削除・Close しても開始が止まらず孤立した転送が残る問題を修正。Close 後の `StartForward` も失敗するように (#105)
- Fix docs for auto-generated rule names (`forward-<N>` for CLI/IPC, `<host>-<type>-<port>` for TUI/GUI) and README Quick Start (`--name web`) / 自動生成ルール名（CLI/IPC は `forward-<N>`、TUI/GUI は `<host>-<type>-<port>`）と README Quick Start（`--name web`）のドキュメントを実装に合わせて修正 (#119)
- Fix docs for `ForwardSession.ID` / `session.list` `id` to the actual `<ruleName>-<UnixNano>` opaque format / `ForwardSession.ID`・`session.list` の `id` を実装の `<ruleName>-<UnixNano>`（不透明な値）に合わせてドキュメント修正 (#120)
- Fix `host.list` losing connection state (every host reported `disconnected`) and always returning `active_forward_count` 0; `SSHManager.LoadHosts` now keeps existing state / `host.list` が接続状態を失い全ホストが `disconnected` になる問題と `active_forward_count` が常に 0 になる問題を修正 (#102)
- Start the host's `auto_connect` forwards on `ssh.connect` / `moleport connect` as documented / `ssh.connect`・`moleport connect` で当該ホストの `auto_connect` ルールを仕様どおり開始 (#118)
- Fix docs for `ssh.connect` idempotency and error codes 1002/1003 to match the implementation / `ssh.connect` の冪等性とエラーコード 1002/1003 のドキュメントを実装に合わせて修正 (#121)
- Fix `moleport start` failing on hosts that need password / passphrase / keyboard-interactive input: register the credential handler and use the 60s connect timeout like `moleport connect` / `moleport start` が認証入力の必要なホストで失敗する問題を修正（`moleport connect` と同じクレデンシャルハンドラーと 60 秒タイムアウトを使用） (#104)
- Deliver daemon event notifications in order through a bounded per-client queue and disconnect clients that stop reading / デーモンのイベント通知をクライアントごとの有界キューで順序どおり配信し、読み取りが滞ったクライアントを切断 (#107)
- Release pending IPC client calls immediately when the connection drops / IPC クライアントの接続断時に保留中の Call を即座に解放 (#110)
- Restore sessions and auto-connect in the background, and force-close after 5 seconds on shutdown (NFR-16) / セッション復元・auto_connect をバックグラウンド化し、停止処理に 5 秒の強制切断タイムアウトを追加 (#128, #116)
- Make `moleport update` wait for the daemon to stop before replacing the binary and restart only a daemon it stopped / `moleport update` がデーモン停止完了を待ってからバイナリを置換し、停止したデーモンのみ再起動 (#111)
- Fix TUI key hints that described forward-list `Enter` / `d` as connect/disconnect: they now read "Toggle start/stop" and "Stop" (the actions never changed), and align the READMEs and command docs with the actual CLI output, flags and key bindings / 転送一覧の `Enter` / `d` のキーヒントが「接続トグル・切断」となっていた問題を「開始/停止トグル・停止」に修正（動作は従来どおり）。README・コマンド仕様を実際の CLI 出力・フラグ・キーバインドに合わせて修正 (#122, #123, #125)
- Fix a connection established while `Disconnect` / `Close` was in progress being registered and leaked; reconnect no longer overrides a user disconnect / Dial 中の `Disconnect` / `Close` 後に接続が登録・残留する問題を修正（再接続もユーザーの切断を上書きしない） (#106)
- Time out SSH keepalive responses (within `keepalive_interval`) so silently dropped connections are detected and reconnected promptly / SSH keepalive の応答にタイムアウトを設け、無音で切れた接続を速やかに検知して再接続 (#108)
- Treat `reconnect.max_retries: 0` as unlimited as documented / `reconnect.max_retries: 0` を仕様どおり無制限として扱う (#113)
- Stop reconnect retries on authentication failure and set the host to `PendingAuth` / 再接続中の認証失敗でリトライを止めて `PendingAuth` にする (#114)
- Apply jitter to the first reconnect delay and never exceed `max_delay` / 再接続の初回遅延にもジッターを付け、待機時間が `max_delay` を超えないようにする (#133)
- Defer key file loading and passphrase prompts until the key is actually needed (ssh-agent and earlier keys are tried first) / 鍵ファイルの読み込みとパスフレーズ要求を実際に必要になるまで遅延（エージェント・先行する鍵を先に試行） (#109)
- Keep rules that the background session restore had not yet processed in `state.yaml` when the daemon stops mid-restore, so they are restored on the next start / セッション復元の途中でデーモンを停止しても、未処理のルールを `state.yaml` に残し次回起動で復元されるよう修正 (#159, #154)
- Do not interrupt an automatic reconnect when a forward is started on a host that is waiting to reconnect: `StartForward` (including `auto_connect`) now fails with a not-connected error instead of opening a manual connection / 再接続待機中のホストで転送を開始しても自動再接続を打ち切らないよう修正（`StartForward`（`auto_connect` 含む）は手動接続せず未接続エラーで失敗する）(#160)
- Fall back to a 1s initial delay when `reconnect.initial_delay` is 0 or negative so `max_retries: 0` no longer retries in a busy loop / `reconnect.initial_delay` が 0 以下のとき 1 秒を使い、`max_retries: 0` で待機なしの再接続ループにならないよう修正 (#161)
- Stop the host's forwards when a dropped connection is detected on a host with `reconnect.enabled: false` (previously they stayed Active on a dead client and were saved to `state.yaml`), save forwards waiting for reconnect (`SessionReconnecting`) to `state.yaml` on daemon stop, and record pending-restore rules before `Start` returns so an immediate stop no longer empties `state.yaml` / `reconnect.enabled: false` のホストで接続断を検知したとき当該ホストの転送を停止するよう修正（死んだクライアントを掴んだまま Active で残り `state.yaml` に保存されていた）。あわせて、デーモン停止時に再接続待ち（`SessionReconnecting`）の転送も `state.yaml` に保存し、`Start` の復帰前に復元待ちルールを記録して起動直後の停止で `state.yaml` が空になる問題を修正 (#162, #163, #164)

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
