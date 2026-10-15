package static

import (
	"io/fs"
	"testing"
)

func TestFS_ContainsDist(t *testing.T) {
	// dist/ は .gitkeep により常に存在する（ビルド生成物がなくても go build が通る）。
	if _, err := fs.Stat(FS(), "dist/.gitkeep"); err != nil {
		t.Errorf("dist/.gitkeep が埋め込まれていない: %v", err)
	}
}
