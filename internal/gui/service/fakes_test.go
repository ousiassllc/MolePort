package service

// staticBackend は固定の Backend を返す BackendSource。
type staticBackend struct {
	b  Backend
	ok bool
}

func (s staticBackend) Backend() (Backend, bool) { return s.b, s.ok }
