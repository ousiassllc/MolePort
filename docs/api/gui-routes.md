# MolePort GUI ルート仕様

## 概要

`moleport-gui` のウィンドウ（WebView）と Go 側の内蔵デーモンの間のインターフェースを定義する。通信は次の 2 種類のみで、TCP/UDP のポートは開かない（NFR-34）。

| 経路 | 方向 | 用途 |
|------|------|------|
| Wails AssetServer（`options.AssetServer.Handler` に渡した `net/http` の `http.Handler`） | WebView → Go | HTML ページ・フラグメントの取得、アクションの実行、静的アセット |
| Wails Events（`runtime.EventsEmit` / `window.runtime.EventsOn`） | Go → WebView | 「再取得すべきトピック」の通知（データは載せない） |

デーモンとクライアント間の IPC（JSON-RPC）は [ipc.md](ipc.md) を参照。GUI はこれを使わない。構成の背景は [gui.md](../architecture/gui.md) を参照する。

## 共通仕様

### Wails AssetServer の制約

| 制約 | 影響 |
|------|------|
| `Assets`（`fs.FS`）を `nil` にして全リクエストを `Handler` に渡す | `/static/*` もルーターが `go:embed` から配信する |
| `/wails/*`（ランタイム・IPC）は Wails が先に処理する | ルーターは `/wails/` 配下を定義しない |
| WebSocket 非対応、レスポンスストリーミングは Windows 非対応 | SSE を使わない。更新は Wails Events + 再取得（D-6） |
| Linux（WebKitGTK）でのリクエストヘッダー・ボディの制限が文書化されている | R-3 で検証。欠落する場合はトークンをフォームフィールドで送る（後述） |
| HTTP リダイレクト（30x）は macOS / Linux で非対応 | リダイレクトを使わない |

### リクエスト

| 項目 | 内容 |
|------|------|
| メソッド | ページ・フラグメントは `GET`。状態変更は `POST`（`DELETE` は削除のみ） |
| `HX-Request` | `/fragments/*` は `HX-Request: true` 必須（なければ `400`）。ブラウザ直接アクセスによる断片表示を防ぐ |
| トークン | `POST` / `DELETE` は `X-MolePort-Token`（`GET /` の `<meta name="mp-token">` の値）が必須。不一致・欠落は `403`。`subtle.ConstantTimeCompare` で照合する |
| ボディ | `application/x-www-form-urlencoded`（HTMX 既定） |
| パスパラメータ | `{name}` は URL デコード済みの値（`http.Request.PathValue`）。ルール名・ホスト名に使える文字は既存の検証に従う |

> R-3 の結果ヘッダーが使えない場合: `X-MolePort-Token` の代わりにフォームフィールド `_token` で受け付ける（ミドルウェアが両方を見る）。仕様上の扱いは同一。

### レスポンスヘッダー（全ルート）

| ヘッダー | 値 |
|---------|----|
| `Content-Security-Policy` | `default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'`（NFR-35。R-4 の結果に応じて最小限の例外を追加） |
| `X-Content-Type-Options` | `nosniff` |
| `Cache-Control` | HTML は `no-store`、`/static/*` は `no-cache` |
| `Content-Type` | HTML は `text/html; charset=utf-8` |

### ステータスコードの方針

HTMX 2 は既定で 4xx/5xx を差し替えない。`app.ts` で `htmx.config.responseHandling` を次のとおり設定する。

| コード | 差し替え | 用途 |
|--------|---------|------|
| `2xx` | する（`204` はしない） | 成功 |
| `422` | する | フォーム検証エラー（フォームを再描画。ステータス上は `error: false` 扱い） |
| `404` / `409` | する | 対象なし・状態競合。本文は **OOB のエラートースト**のみ（メイン領域は変更しない） |
| `400` / `403` | しない（`error: true`） | 不正リクエスト・トークン不一致。`htmx:responseError` → 汎用エラートースト |
| `5xx` | しない（`error: true`） | 内部エラー。`htmx:responseError` → 汎用エラートースト |

エラー本文に内部情報（スタックトレース、ファイルパス）を含めない。業務エラーは Core のエラー型（`NotFoundError` / `AlreadyExistsError` / ポート競合 / 認証失敗）をローカライズ済みメッセージに変換して返す。

### OOB トースト

アクションルートは副作用を OOB（`hx-swap-oob="beforeend:#toast-stack"`）で返す。メイン領域の更新には OOB を使わない（HALT の方針）。

```html
<div hx-swap-oob="beforeend:#toast-stack">
  <div id="toast-7" class="toast toast-error" role="alert">ルール 'prod-web' の開始に失敗: port 8080 is in use</div>
</div>
```

トーストは Templ の `atoms.Toast(kind, message)`（`kind`: `info` / `success` / `error`）。成功・情報は 5 秒で自動消去（`app.ts` が `#toast-stack` への追加を監視して消去）、エラーは `data-dismiss-toast` の閉じるボタンで手動消去する。

## ルート一覧

| メソッド | パス | 種別 | 説明 |
|---------|------|------|------|
| GET | `/` | ページ | ダッシュボード全体（layout + 全フラグメント） |
| GET | `/static/{path...}` | 静的 | `app.js`、`htmx.min.js`、`app.css`、アイコン |
| GET | `/fragments/header` | フラグメント | ヘッダー（バージョン・稼働状態・更新バッジ・設定ボタン） |
| GET | `/fragments/hosts` | フラグメント | ホストパネル |
| GET | `/fragments/forwards` | フラグメント | 転送パネル |
| GET | `/fragments/status` | フラグメント | ステータスバー |
| GET | `/fragments/credential` | フラグメント | クレデンシャルダイアログ（待機中の要求がなければ空） |
| GET | `/fragments/toasts` | フラグメント | 非同期に発生した通知（接続・転送開始の失敗、引き継ぎ完了など）のキューを取り出し、OOB トーストとして返す |
| GET | `/fragments/forward-form` | フラグメント | 転送追加フォーム（モーダル内容） |
| GET | `/fragments/forward-delete` | フラグメント | 削除確認ダイアログ |
| GET | `/fragments/settings` | フラグメント | 設定フォーム |
| POST | `/actions/forwards` | アクション | 転送ルールを追加して開始 |
| POST | `/actions/forwards/{name}/start` | アクション | 転送を開始 |
| POST | `/actions/forwards/{name}/stop` | アクション | 転送を停止 |
| DELETE | `/actions/forwards/{name}` | アクション | 転送を停止して削除 |
| POST | `/actions/hosts/{name}/connect` | アクション | SSH 接続 |
| POST | `/actions/hosts/{name}/disconnect` | アクション | SSH 切断 |
| POST | `/actions/credential/{id}` | アクション | クレデンシャルを送信 |
| POST | `/actions/credential/{id}/cancel` | アクション | クレデンシャル入力をキャンセル |
| POST | `/actions/settings` | アクション | 設定を保存 |
| POST | `/actions/update/open` | アクション | 新バージョンのリリースページを既定ブラウザで開く |

JSON API ルート（HALT の API ルート）は初期リリースでは提供しない（Lit コンポーネントを持たないため。D-10）。

## ページ

### `GET /`

フルページを返す。`BaseLayout` に以下を含める。

- `<html lang="{ja|en}" data-theme="{system|dark|light}" data-accent="{...}">`（`config.yaml` の保存値）
- `<meta name="mp-token" content="{起動ごとのランダムトークン}">`
- `<script type="module" src="/static/dist/js/app.js">`、`<script src="/static/dist/js/htmx.min.js">`、`<link rel="stylesheet" href="/static/dist/css/app.css">`
- `#header` / `#hosts-panel` / `#forwards-panel` / `#status-bar` / `#modal-root` / `#credential-root` / `#toast-stack`
- ライフサイクル状態（`starting` / `ready` / `failed`）に応じたバナー。`ready` でない間はアクションボタンを描画しない

各領域は初期表示でインライン展開し、更新トリガーを持つ。

```html
<section id="forwards-panel"
         hx-get="/fragments/forwards"
         hx-trigger="mp:forwards from:body, mp:all from:body"
         hx-swap="innerHTML">
  <!-- Templ: organisms.ForwardPanel(views) -->
</section>
```

`mp:*` は `app.ts` が Wails Events から発火するカスタム DOM イベント（後述）。

## フラグメント

すべて `GET`、`HX-Request: true` 必須、読み取り専用（副作用なし）。

| パス | クエリ | 内容 | 主な HATEOAS 出し分け |
|------|--------|------|---------------------|
| `/fragments/header` | — | バージョン、PID、稼働表示、更新バッジ（`update_check` 有効かつ新版あり） | 更新バッジは更新があるときのみ。クリックで `POST /actions/update/open` |
| `/fragments/hosts` | — | ホスト行（`HostView`）。0 件は案内文 | 接続 / 切断 / 転送追加ボタン（`HostActions`） |
| `/fragments/forwards` | — | 転送行（`ForwardView`）。0 件は案内と追加ボタン | 開始 / 停止 / 削除ボタン（`ForwardActions`） |
| `/fragments/status` | — | ホスト数・接続数・転送数・アクティブ数 | — |
| `/fragments/credential` | — | 待機中の先頭の `CredentialView`。`password` / `passphrase` は 1 入力、`keyboard-interactive` は `Prompts` 数だけ入力。`Echo=false` は `type="password"` | 待機中の要求がなければ空の `#credential-root` 内容。`app.ts` は表示中のダイアログの要求 ID をリクエストヘッダー `X-MolePort-Credential-Shown` で送り、先頭の要求と一致すれば `204`（差し替えない。フォーカス復帰の `mp:all` で入力途中の値を失わないため） |
| `/fragments/toasts` | — | `NotificationQueue`（GUI Service が保持するメモリ上の FIFO）を **取り出して（drain）** OOB の `beforeend:#toast-stack` で返す。空なら `204`。取り出し時にキューを消すため、このフラグメントのみ読み取りで状態が変わる | — |
| `/fragments/forward-form` | `host`（任意、選択済みにする） | 転送追加フォーム。種別（local / remote / dynamic）の選択で項目を出し分ける（`hx-get` で再取得） | `type=dynamic` ではリモート項目を描画しない。`type=remote` のときのみバインドアドレスを描画 |
| `/fragments/forward-delete` | `name` | 削除確認（ルール名表示）。「削除」は `DELETE /actions/forwards/{name}` | — |
| `/fragments/settings` | — | 言語 / テーマ（base・accent スウォッチ）/ アップデートチェックのフォーム | — |

- ホスト行・転送行の要素には安定した `id`（`host-{name}`、`fwd-{name}`、`fwd-{name}-start` 等）を付与する（ライブ更新でフォーカスを保つため。gui.md「ティッカー」）。`name` を `id` に使う際は英数字以外を `_` + 16 進でエスケープする（ID 衝突を避けるため `name` 自体も `data-name` 属性で保持する）
- 稼働状態（`starting` / `failed`）のとき、ホスト・転送パネルは見出しのみを返し一覧（アクションボタン）を描画しない
- `#toast-stack` は `hx-get="/fragments/toasts" hx-trigger="mp:toast from:body, mp:all from:body" hx-swap="none"` を持ち、`toast` / `all` トピックでキューを取り出す

## アクション

すべてトークン必須（`403`）。更新後のフラグメントを本文に含め、副作用はトーストを OOB で返す。

### `POST /actions/forwards` — 転送ルールの追加

| フィールド | 型 | 必須 | 内容 |
|-----------|----|------|------|
| `host` | string | ✓ | SSH ホスト名（`Host` エイリアス） |
| `type` | string | ✓ | `local` / `remote` / `dynamic` |
| `name` | string | — | ルール名。空なら `{host}-{type}-{local_port}` を自動生成（CLI/TUI と同じ） |
| `local_port` | string | ✓ | 1〜65535 |
| `remote_host` | string | local / remote で任意 | 空なら `localhost` |
| `remote_port` | string | local / remote で任意 | 空なら `local_port` と同値（F-50） |
| `remote_bind_addr` | string | remote で任意 | 空なら `127.0.0.1`（F-51） |
| `auto_connect` | `on` / なし | — | |

処理: 検証 → `ForwardManager.AddRule` → `core.PersistForwardRules`（config.yaml 保存）→ `StartForward`（非同期。認証が必要なら UC-31）。

| 結果 | ステータス | 本文 |
|------|-----------|------|
| 成功 | `200` | 転送パネルの更新後フラグメント + OOB トースト（success）+ `HX-Trigger: mp:close-modal` |
| 検証エラー（ポート範囲外・必須欠落・名前重複・未知ホスト） | `422` | `ForwardForm` の再描画（`FormErrors` 付き、入力値保持）。`HX-Retarget: #modal-root` / `HX-Reswap: innerHTML` でモーダル内に差し替え |
| ルール追加は成功したが開始に失敗（ポート競合等） | — | 開始は非同期のため成功時と同じ `200`（当該行は「開始中…」）。失敗は `Error` 状態（`LastError`）+ `NotificationQueue` のエラートーストで後から通知する。ルールは保存済み |

ポート競合のメッセージには空きポートの候補を含める（UC-28）。

### `POST /actions/forwards/{name}/start`・`/stop`、`DELETE /actions/forwards/{name}`

| ルート | 前提（HATEOAS） | 処理 |
|-------|----------------|------|
| `start` | `CanStart`（`Stopped` / `Error`） | `StartForward(name, cb)`（非同期。`cb` は `CredentialBroker`） |
| `stop` | `CanStop`（`Active` / `Reconnecting`） | `StopForward(name)` |
| `DELETE` | 常に可 | `DeleteRule(name)` → `core.PersistForwardRules` |

| 結果 | ステータス | 本文 |
|------|-----------|------|
| 成功 | `200` | 転送パネルの更新後フラグメント（+ 削除時は `HX-Trigger: mp:close-modal`） |
| ルールなし | `404` | OOB トースト（error）。`mp:forwards` で再取得させる |
| 状態不一致（既に Active、既に Stopped 等） | `409` | OOB トースト（info）+ 現状態でのパネル再取得は `HX-Trigger: mp:forwards` で行う |
| 開始・接続の非同期失敗（認証失敗・ポート競合・接続エラー） | — | レスポンスには含めない。`ForwardEventError` / `SSHEventError` を EventBridge が `NotificationQueue` に積み、`toast` トピックで `/fragments/toasts` を再取得させて表示する。状態表示（`Error` バッジ・`LastError`）は `forwards` / `hosts` の再取得で反映される。アクション直後の同期的な失敗（ルールなし・状態不一致）のみレスポンスの OOB で返す |

### `POST /actions/hosts/{name}/connect`・`/disconnect`

| ルート | 前提 | 処理 |
|-------|------|------|
| `connect` | `CanConnect`（`Disconnected` / `Error` / `PendingAuth`） | `ConnectWithCallback(name, cb)`（非同期）。成功時に当該ホストの auto_connect ルールを開始（UC-4） |
| `disconnect` | `CanDisconnect`（`Connected` / `Reconnecting`） | `Disconnect(name)`（再接続ループ中の切断は UC-8 に従う）。当該ホストのアクティブ転送は停止する |

成功時 `200`: ホストパネルの更新後フラグメント（接続中は「接続中…」バッジとスピナー）。ホストなし `404`、状態不一致 `409`（トースト）。

### `POST /actions/credential/{id}`

| フィールド | 型 | 内容 |
|-----------|----|------|
| `value` | string | `password` / `passphrase` の値 |
| `answers[]` | string の配列 | `keyboard-interactive` の各プロンプトへの回答（`Prompts` と同数・同順） |

`CredentialBroker.Resolve(id, credential.Input)` を呼ぶ。`id` が存在しない・解決済み・タイムアウト済みは `409` + OOB トースト（「入力の有効期限が切れました」）+ `HX-Trigger: mp:credential`。成功時は `200` で、次の待機要求があれば `CredentialDialog` を、なければ空を返し、`HX-Trigger: mp:close-modal` は付けない（`#credential-root` の開閉は `/fragments/credential` の内容に従う）。

- 本文（クレデンシャル）はログ出力しない。リクエストログのミドルウェアはこのルートを除外する（NFR-36）
- `Content-Type` が `application/x-www-form-urlencoded` 以外は `415`、不正なエンコーディングは `400`
- 値は文字列化せずバイト列のまま解析し、`Resolve` に渡した後にハンドラ内のバッファ（ボディ・各値）をゼロ化する

### `POST /actions/credential/{id}/cancel`

`CredentialBroker.Cancel(id)`（`CredentialResponse{Cancelled: true}`）。応答は上記と同じ。30 秒のタイムアウトはサーバー側（`core.CredentialTimeout`）で発火し、解決後に `credential` トピックが通知されてダイアログが閉じる。

### `POST /actions/settings`

| フィールド | 値 |
|-----------|----|
| `language` | `ja` / `en` |
| `theme_base` | `system` / `dark` / `light` |
| `theme_accent` | `violet` / `blue` / `green` / `cyan` / `orange` |
| `update_check_enabled` | `on` / なし |

処理: 検証 → `ConfigManager.UpdateConfig`（`language`、`gui.theme`、`update_check.enabled` のみ変更）→ `i18n.SetLang` → トレイメニュー再構築 → `VersionChecker` の有効/無効反映。

| 結果 | ステータス | 本文 |
|------|-----------|------|
| 成功 | `200` | `HX-Refresh` は使わず `HX-Trigger: mp:close-modal, mp:all`（全フラグメントを再取得し、`<html>` の `lang` / `data-theme` / `data-accent` は `app.ts` が `HX-Trigger-After-Swap` で受けた値で更新する）+ OOB トースト |
| 検証エラー | `422` | `SettingsForm` の再描画（`FormErrors` 付き） |

> `<html>` 属性の更新: レスポンスヘッダー `HX-Trigger: {"mp:settings-applied": {"lang": "ja", "theme": "dark", "accent": "violet"}}` を返し、`app.ts` が `document.documentElement` の属性を書き換える（全画面遷移・`HX-Refresh` を避けるため）。

### `POST /actions/update/open`

サーバーが `VersionChecker` のキャッシュからリリース URL を取得し、`https://github.com/ousiassllc/moleport/releases/` で始まる場合のみ `Opener`（Shell が `runtime.BrowserOpenURL` で実装）に渡す。URL はクライアントから受け取らない。成功 `204`、更新情報なし `409`。

## Wails Events

| イベント名 | 方向 | ペイロード | 内容 |
|-----------|------|-----------|------|
| `moleport:changed` | Go → JS | `{"topics": ["hosts", "forwards", ...]}` | 再取得すべきトピック。データ・クレデンシャルを含めない（NFR-36） |

`app.ts` の変換:

```ts
window.runtime.EventsOn('moleport:changed', ({ topics }: { topics: string[] }) => {
  for (const t of topics) htmx.trigger(document.body, `mp:${t}`);
})
window.addEventListener('focus', () => htmx.trigger(document.body, 'mp:all'));
```

| トピック | 受信側（`hx-trigger`） |
|---------|----------------------|
| `hosts` | `#hosts-panel` |
| `forwards` | `#forwards-panel` |
| `status` | `#status-bar` |
| `header` | `#header` |
| `credential` | `#credential-root`（`/fragments/credential`。内容があればモーダル表示） |
| `toast` | `#toast-stack` の取り込み（`/fragments/toasts`、`hx-swap="none"`。OOB のみ使用） |
| `settings` | 開いている設定フォーム（任意） |
| `all` | 上記すべて |

`ModalRoot` は転送追加フォーム・削除確認・設定を `hx-get` で `#modal-root` に差し込む。クレデンシャルダイアログは別の `#credential-root` に差し込み、他のモーダルより前面（top layer）に重ねて表示する（先に開いていたモーダルは閉じないため入力途中の値は失われない）。トレイ・復元起因の要求はウィンドウを前面表示した上でダイアログを出す。

## クライアント JS（`app.ts`）の契約

| 責務 | 内容 |
|------|------|
| Events 変換 | 上記 |
| トークン付与 | `htmx:configRequest` で `X-MolePort-Token`（`<meta name="mp-token">`）を付与 |
| `responseHandling` | 「ステータスコードの方針」の表 |
| エラートースト | `htmx:responseError` / `htmx:sendError` で汎用エラートーストを DOM に追加（サーバーが応答できない場合に限る唯一のクライアント側 UI 生成） |
| モーダル | ネイティブ `<dialog>` を `showModal()` で開く。`mp:close-modal`（`HX-Trigger`）・`[data-close-modal]`・`Esc` で `#modal-root` を空にする。クレデンシャルダイアログの `Esc` はキャンセル扱い（`/actions/credential/{id}/cancel`） |
| テーマ | `mp:settings-applied` で `<html>` 属性を更新。設定フォームのスウォッチ選択はプレビューとして `data-theme` / `data-accent` を即時反映し、キャンセル時に元の値へ戻す |

Lit コンポーネントは初期リリースでは存在しない（D-10）。HTMX ↔ Lit 境界ルールは Lit 追加時に適用する。

## 改訂履歴

| 版 | 日付 | 変更内容 | 変更理由 |
|----|------|---------|---------|
| 1.0 | 2026-10-15 | 新規作成 | #84 Wails 版 GUI アプリケーション |
| 1.1 | 2026-10-15 | `#toast-stack` の取り込みトリガーと稼働状態時のパネル描画を実装に合わせて明記 | #88 M3-1 ダッシュボード |
| 1.2 | 2026-10-15 | 操作系の実装反映: `#credential-root`（クレデンシャルダイアログを他モーダルに重ねる）、`X-MolePort-Credential-Shown` による差し替え抑止、クレデンシャルルートの `415` / `400`、422 の差し替え先指定（`HX-Retarget`）、転送追加時の開始失敗の通知経路、モーダルの開閉方法を追記 | #89 M3-2 操作系 |
