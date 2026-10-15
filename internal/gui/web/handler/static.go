package handler

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"time"
)

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
	// 更新時刻は返さない（Cache-Control: no-cache はミドルウェアが付与）。Content-Type は拡張子から決まる。
	http.ServeContent(w, r, path.Base(name), time.Time{}, rs)
}
