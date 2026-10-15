package handler

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// contentTypes は配信するアセットの Content-Type。http.ServeContent は拡張子から
// mime.TypeByExtension で決めるが、Windows ではレジストリの登録値（.js が text/plain 等）に
// 依存する。nosniff 下の <script type="module"> は JavaScript の MIME でないと読み込まれないため、
// OS 非依存になるよう固定の表で明示する。表にない拡張子は ServeContent に任せる。
var contentTypes = map[string]string{
	".js":    "text/javascript; charset=utf-8",
	".mjs":   "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".map":   "application/json",
	".json":  "application/json",
	".html":  "text/html; charset=utf-8",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".ico":   "image/x-icon",
	".woff":  "font/woff",
	".woff2": "font/woff2",
}

// Static は fsys（go:embed、テストでは fstest.MapFS）から /static/{path...} を配信する。
// AssetServer はリダイレクトを扱えないため、ディレクトリや index.html の 30x は返さず 404 にする。
type Static struct {
	fsys fs.FS
}

// NewStatic は fsys を配信元とする Static を返す。
func NewStatic(fsys fs.FS) *Static { return &Static{fsys: fsys} }

// ServeHTTP は PathValue("path") のファイルを返す。
func (s *Static) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("path")
	if name == "" || !fs.ValidPath(name) {
		http.NotFound(w, r)
		return
	}
	f, err := s.fsys.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	// 更新時刻は返さない（Cache-Control: no-cache はミドルウェアが付与）。
	if ct, ok := contentTypes[strings.ToLower(path.Ext(name))]; ok {
		w.Header().Set("Content-Type", ct)
	}
	http.ServeContent(w, r, path.Base(name), time.Time{}, rs)
}
