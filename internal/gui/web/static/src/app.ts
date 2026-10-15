// MolePort GUI のクライアント JS。HALT の原則（クライアント状態を持たない）に従い、
// 責務は次の点に限定する（docs/architecture/gui.md「JS の責務」）。
//   1. Wails Events（moleport:changed）→ htmx.trigger(mp:<topic>)
//   2. htmx:configRequest での X-MolePort-Token 付与
//   3. htmx:responseError / htmx:sendError の汎用エラートースト（3b: トーストの自動消去・手動消去）
//   4. テーマのプレビュー（data-theme / data-accent の即時切り替え）
//   5. モーダル（ネイティブ <dialog>）の開閉
//
// ライブ更新（1 秒ごとのフラグメント差し替え）でフォーカスを保つため、操作可能要素には
// サーバーが安定した id（host-{name}-connect、fwd-{name}-stop 等）を付与する。
// HTMX は差し替え前後で同一 id の要素へフォーカスを復元するため、クライアント側に状態は持たない。

// htmx.min.js はグローバルとして先に読み込まれる（CSP: script-src 'self'）。
declare const htmx: {
  config: { responseHandling: { code: string; swap: boolean; error?: boolean }[] };
  trigger(target: Element | string, event: string, detail?: unknown): void;
};

interface WailsRuntime {
  EventsOn(name: string, callback: (data: { topics: string[] }) => void): () => void;
}

interface HTMXConfigRequestDetail {
  headers: Record<string, string>;
}

const TOKEN_HEADER = "X-MolePort-Token";

// ステータスコードの方針（docs/api/gui-routes.md）。
// 422（フォーム検証エラー）・404 / 409（OOB トーストのみの本文）は差し替える。400 / 403 / 5xx は差し替えずエラー扱い。
htmx.config.responseHandling = [
  { code: "204", swap: false },
  { code: "[23]..", swap: true },
  { code: "422", swap: true },
  { code: "404", swap: true },
  { code: "409", swap: true },
  { code: "[45]..", swap: false, error: true },
  { code: "...", swap: true },
];

// 1. Wails Events → htmx トリガー。
function bridgeWailsEvents(): void {
  const runtime = (window as unknown as { runtime?: WailsRuntime }).runtime;
  runtime?.EventsOn("moleport:changed", ({ topics }) => {
    for (const topic of topics) htmx.trigger(document.body, `mp:${topic}`);
  });
  window.addEventListener("focus", () => htmx.trigger(document.body, "mp:all"));
}

// 2. 状態変更リクエストへのトークン付与（トークンは <meta name="mp-token"> から取得）。
document.body.addEventListener("htmx:configRequest", (e) => {
  const token = document.querySelector<HTMLMetaElement>('meta[name="mp-token"]')?.content;
  if (token) (e as CustomEvent<HTMXConfigRequestDetail>).detail.headers[TOKEN_HEADER] = token;
});

// 3. サーバーが応答できない場合に限る、唯一のクライアント側 UI 生成（汎用エラートースト）。
function showErrorToast(): void {
  const stack = document.getElementById("toast-stack");
  if (!stack) return;
  const toast = document.createElement("div");
  toast.className = "mp-toast";
  toast.dataset.kind = "error";
  toast.setAttribute("role", "alert");
  const message = document.createElement("span");
  message.textContent = stack.dataset.errorMessage ?? "";
  const close = document.createElement("button");
  close.type = "button";
  close.className = "mp-toast-close";
  close.setAttribute("data-dismiss-toast", "");
  close.setAttribute("aria-label", stack.dataset.dismissLabel ?? "");
  close.textContent = "\u00d7";
  toast.append(message, close);
  stack.append(toast);
}
document.body.addEventListener("htmx:responseError", showErrorToast);
document.body.addEventListener("htmx:sendError", showErrorToast);

// 3b. トーストの消去。成功・情報は 5 秒で自動消去し、エラーは閉じるボタン（data-dismiss-toast）で手動消去する。
//     トースト本体は常にサーバーが描画する（OOB で #toast-stack の末尾へ追記される）。
const TOAST_AUTO_DISMISS_MS = 5000;

function watchToastStack(): void {
  const stack = document.getElementById("toast-stack");
  if (!stack) return;
  new MutationObserver((records) => {
    for (const record of records) {
      for (const node of record.addedNodes) {
        if (node instanceof HTMLElement && node.dataset.kind !== "error") {
          setTimeout(() => node.remove(), TOAST_AUTO_DISMISS_MS);
        }
      }
    }
  }).observe(stack, { childList: true });
  stack.addEventListener("click", (e) => {
    const target = e.target as Element;
    if (target.closest("[data-dismiss-toast]")) target.closest(".mp-toast")?.remove();
  });
}

// 4. テーマのプレビュー。サーバー往復なしで <html> の属性を書き換える（保存値はサーバーが次回レンダリングで出力）。
//    mp:theme-preview … 設定フォームのスウォッチ選択（detail: {theme, accent}）
//    mp:settings-applied … 保存成功時に HX-Trigger で受け取る確定値（detail: {lang, theme, accent}）
interface ThemeDetail {
  lang?: string;
  theme: string;
  accent: string;
}

function applyTheme({ lang, theme, accent }: ThemeDetail): void {
  const root = document.documentElement;
  if (lang) root.lang = lang;
  root.dataset.theme = theme;
  root.dataset.accent = accent;
}
for (const name of ["mp:theme-preview", "mp:settings-applied"]) {
  document.body.addEventListener(name, (e) => applyTheme((e as CustomEvent<ThemeDetail>).detail));
}

// 5. モーダル。#modal-root（転送追加フォーム・削除確認・設定）と #credential-root（クレデンシャルダイアログ）に
//    差し込まれた <dialog> を showModal() で開く（フォーカストラップと Esc はネイティブ）。
//    - mp:close-modal（HX-Trigger）と [data-close-modal] のクリックで #modal-root を空にする。
//    - Esc で閉じたとき（close イベント）も #modal-root を空にする。
//    - クレデンシャルダイアログの Esc はキャンセル扱い（[data-credential-cancel] をクリックして POST する）。
//    - #credential-root は別の領域にし、先に開いているフォームの入力を壊さずに前面（top layer）へ重ねる。
function bridgeModals(): void {
  const modalRoot = document.getElementById("modal-root");
  const credentialRoot = document.getElementById("credential-root");

  for (const root of [modalRoot, credentialRoot]) {
    if (!root) continue;
    const openNew = () => root.querySelectorAll("dialog:not([open])").forEach((d) => (d as HTMLDialogElement).showModal());
    new MutationObserver(openNew).observe(root, { childList: true });
  }

  const closeModal = () => modalRoot?.replaceChildren();
  document.body.addEventListener("mp:close-modal", closeModal);
  document.body.addEventListener("click", (e) => {
    if ((e.target as Element).closest("[data-close-modal]")) closeModal();
  });
  modalRoot?.addEventListener("close", closeModal, true);
  credentialRoot?.addEventListener(
    "cancel",
    (e) => {
      e.preventDefault();
      credentialRoot.querySelector<HTMLElement>("[data-credential-cancel]")?.click();
    },
    true,
  );
}

// /fragments/credential の再取得時に、表示中のダイアログの要求 ID を伝える（サーバーが同一要求なら差し替えない）。
document.body.addEventListener("htmx:configRequest", (e) => {
  const detail = (e as CustomEvent<HTMXConfigRequestDetail & { path: string }>).detail;
  if (detail.path !== "/fragments/credential") return;
  const shown = document.querySelector<HTMLElement>("#credential-root [data-request-id]")?.dataset.requestId;
  if (shown) detail.headers["X-MolePort-Credential-Shown"] = shown;
});

bridgeWailsEvents();
watchToastStack();
bridgeModals();
