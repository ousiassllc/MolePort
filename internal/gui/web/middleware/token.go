package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
)

const (
	// TokenHeader は状態変更リクエストに付与するトークンのヘッダー名。
	TokenHeader = "X-MolePort-Token" //nolint:gosec // ヘッダー名であり秘密情報ではない

	tokenBytes = 32
)

// NewToken は crypto/rand による 32 バイトのランダムトークン（16 進 64 文字）を生成する。
// 起動ごとに 1 回生成し、GET / の <meta name="mp-token"> に埋め込む。
func NewToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Token は状態変更メソッド（GET / HEAD 以外）に対し、TokenHeader を
// subtle.ConstantTimeCompare で照合する。欠落・不一致は 403。
// token が空の場合は全ての状態変更リクエストを拒否する。
func Token(token string) Middleware {
	expected := []byte(token)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}
			got := []byte(r.Header.Get(TokenHeader))
			if len(expected) == 0 || subtle.ConstantTimeCompare(got, expected) != 1 {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
