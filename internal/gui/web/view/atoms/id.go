package atoms

import (
	"strconv"
	"strings"
)

// ElementID は prefix と name から DOM の id を組み立てる。
// name の英数字以外（"_" を含む）は "_" + 2 桁の 16 進（バイト単位）にエスケープし、
// 異なる name が同じ id に衝突しないようにする。
// ライブ更新の差し替えでも同じ id が付くため、HTMX がフォーカスを復元できる。
func ElementID(prefix, name string) string {
	var b strings.Builder
	b.WriteString(prefix)
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('_')
		if c < 0x10 {
			b.WriteByte('0')
		}
		b.WriteString(strconv.FormatUint(uint64(c), 16))
	}
	return b.String()
}
