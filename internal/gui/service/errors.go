package service

import "errors"

var (
	// ErrStateConflict は操作が現在の状態で実行できない（HATEOAS の Actions が false）ことを表す。
	// ハンドラが 409 に変換する。
	ErrStateConflict = errors.New("state conflict")
	// ErrNotReady は内蔵デーモンが起動していない間に操作が要求されたことを表す。ハンドラが 409 に変換する。
	ErrNotReady = errors.New("daemon not ready")
)
