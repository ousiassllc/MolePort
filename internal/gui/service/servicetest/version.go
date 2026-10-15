package servicetest

import (
	"sync"

	"github.com/ousiassllc/moleport/internal/core"
)

// Version は service.VersionSource のフェイク。Result を Cached で返し、SetEnabled の呼び出しを記録する。
type Version struct {
	Result core.VersionCheckResult
	OK     bool

	mu      sync.Mutex
	enabled []bool
}

func (v *Version) Cached() (core.VersionCheckResult, bool) { return v.Result, v.OK }

func (v *Version) SetEnabled(enabled bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.enabled = append(v.enabled, enabled)
}

// EnabledCalls は SetEnabled に渡された値を呼び出し順に返す。
func (v *Version) EnabledCalls() []bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]bool(nil), v.enabled...)
}
