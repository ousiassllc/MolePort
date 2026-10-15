package handler

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/a-h/templ"

	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/gui/service/credential"
	"github.com/ousiassllc/moleport/internal/gui/web/view/organisms"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// maxCredentialBody はクレデンシャルの POST ボディの上限（バイト）。
const maxCredentialBody = 64 << 10

// Credential はクレデンシャルダイアログのフラグメントとアクション。
// 値は POST ボディ（application/x-www-form-urlencoded）のみで受け取り、文字列（ログ・URL に出うる形）に
// せずバイト列のまま Service へ渡す。Resolve が引き渡し後にバッファをゼロ化し、ここでもボディをゼロ化する（NFR-36）。
type Credential struct {
	cred credential.Prompter
}

// NewCredential は Credential を返す。
func NewCredential(cred credential.Prompter) *Credential { return &Credential{cred: cred} }

// CredentialShownHeader は #credential-root に表示中のダイアログの要求 ID を app.ts が付けるリクエストヘッダー。
// 同じ要求のダイアログが既に表示されているときは差し替えない（window フォーカス復帰の mp:all で入力途中の値を失わないため）。
const CredentialShownHeader = "X-MolePort-Credential-Shown" //nolint:gosec // ヘッダー名であり秘密情報ではない

// Dialog は GET /fragments/credential。待機中の先頭の要求があればダイアログ、なければ空を返す。
// 先頭の要求が CredentialShownHeader と同じなら 204（差し替えなし）。
func (c *Credential) Dialog(w http.ResponseWriter, r *http.Request) {
	if v, ok := c.cred.Pending(); ok && v.RequestID == r.Header.Get(CredentialShownHeader) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	render(w, r, http.StatusOK, c.current())
}

// current は先頭の待機中要求のダイアログ。なければ何も描画しない（#credential-root を空にする）。
func (c *Credential) current() templ.Component {
	if v, ok := c.cred.Pending(); ok {
		return organisms.CredentialDialog(v)
	}
	return templ.NopComponent
}

// Resolve は POST /actions/credential/{id}。応答は次の待機要求のダイアログ（なければ空）。
func (c *Credential) Resolve(w http.ResponseWriter, r *http.Request) {
	in, ok := readCredential(w, r)
	if !ok {
		return
	}
	// Resolve が引き渡し後に in のバッファをゼロ化する。
	err := c.cred.Resolve(r.PathValue("id"), in)
	c.respond(w, r, err)
}

// Cancel は POST /actions/credential/{id}/cancel。
func (c *Credential) Cancel(w http.ResponseWriter, r *http.Request) {
	c.respond(w, r, c.cred.Cancel(r.PathValue("id")))
}

func (c *Credential) respond(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, credential.ErrUnknownRequest) {
		// 解決済み・タイムアウト済み。ダイアログを現状に合わせて取り直させる。
		reswapNone(w, eventCredential)
		render(w, r, http.StatusConflict, toastOOB(service.ToastInfo, i18n.T("gui.credential.expired")))
		return
	}
	render(w, r, http.StatusOK, c.current())
}

// readCredential は application/x-www-form-urlencoded のボディから value / answers[] を取り出す。
// 不正なリクエストには 4xx を書いて ok=false を返す。
func readCredential(w http.ResponseWriter, r *http.Request) (credential.Input, bool) {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/x-www-form-urlencoded" {
		http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)
		return credential.Input{}, false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxCredentialBody))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return credential.Input{}, false
	}
	in, err := parseCredentialForm(body)
	clear(body)
	if err != nil {
		in.Zero()
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return credential.Input{}, false
	}
	return in, true
}

// parseCredentialForm はフォームのボディをバイト列のまま解析し、各値を新しいバッファへコピーして返す。
// body 自体は呼び出し側がゼロ化する。
func parseCredentialForm(body []byte) (credential.Input, error) {
	var in credential.Input
	for _, pair := range bytes.Split(body, []byte("&")) {
		if len(pair) == 0 {
			continue
		}
		rawKey, rawValue, _ := bytes.Cut(pair, []byte("="))
		key, err := unescapeForm(rawKey)
		if err != nil {
			in.Zero()
			return credential.Input{}, err
		}
		value, err := unescapeForm(rawValue)
		if err != nil {
			clear(key)
			in.Zero()
			return credential.Input{}, err
		}
		switch string(key) {
		case "value":
			clear(in.Value)
			in.Value = value
		case "answers[]":
			in.Answers = append(in.Answers, value)
		default:
			clear(value)
		}
	}
	return in, nil
}

// errBadEscape は不正なパーセントエンコーディング。
var errBadEscape = errors.New("invalid percent-encoding")

// unescapeForm は application/x-www-form-urlencoded の 1 要素（'+' は空白、%XX は 1 バイト）を新しいバッファへ復号する。
func unescapeForm(s []byte) ([]byte, error) {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '+':
			out = append(out, ' ')
		case '%':
			if i+2 >= len(s) {
				clear(out)
				return nil, errBadEscape
			}
			hi, okHi := unhex(s[i+1])
			lo, okLo := unhex(s[i+2])
			if !okHi || !okLo {
				clear(out)
				return nil, errBadEscape
			}
			out = append(out, hi<<4|lo)
			i += 2
		default:
			out = append(out, c)
		}
	}
	return out, nil
}

// unhex は 16 進数字 1 文字の値を返す。16 進数字でなければ ok=false。
func unhex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
