package handler

import (
	"bytes"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
)

// render は Templ コンポーネントをバッファへ描画してから書き出す。
// 描画エラー時に部分的な HTML を返さず、内部情報を含まない 500 を返す。
func render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	var buf bytes.Buffer
	if err := c.Render(r.Context(), &buf); err != nil {
		slog.Error("gui: render failed", "path", r.URL.Path, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
