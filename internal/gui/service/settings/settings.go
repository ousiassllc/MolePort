// Package settings は GUI の設定画面（UC-34）と新バージョンのリリースページ起動（UC-35）の Service。
package settings

import (
	"errors"
	"strings"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/core/guiconfig"
	"github.com/ousiassllc/moleport/internal/gui/service"
	"github.com/ousiassllc/moleport/internal/i18n"
)

// ErrNoUpdate は開くべき新バージョンがない（update_check 無効・新版なし・URL 不正）ことを表す。ハンドラが 409 に変換する。
var ErrNoUpdate = errors.New("no update available")

// Opener は URL を既定のブラウザで開く。Shell が runtime.BrowserOpenURL で実装する。
type Opener interface {
	OpenURL(url string)
}

// Settings は設定の表示・保存とリリースページの起動。
type Settings interface {
	// View は保存済みの設定を返す。デーモン未起動なら service.ErrNotReady。
	View() (service.SettingsView, error)
	// Save は入力を検証して保存し、言語と update_check を即時に反映する。
	// 検証エラーはフィールド別の FormErrors で返す（その場合 error は nil）。
	Save(in service.SettingsView) (service.FormErrors, error)
	// OpenUpdate は新バージョンのリリースページを開く。URL はキャッシュから取得しクライアントから受け取らない。
	// 開くべき更新がなければ ErrNoUpdate。
	OpenUpdate() error
}

// Service は Core の ConfigManager / VersionChecker を使う Settings の実装。
type Service struct {
	src  service.BackendSource
	open Opener
}

var _ Settings = (*Service)(nil)

// New は Service を返す。
func New(src service.BackendSource, open Opener) *Service {
	return &Service{src: src, open: open}
}

func (s *Service) backend() (service.Backend, error) {
	b, ok := s.src.Backend()
	if !ok {
		return service.Backend{}, service.ErrNotReady
	}
	return b, nil
}

// View は Settings を実装する。言語は解決済みの表示言語（config 未設定なら環境変数・既定値）を返す。
func (s *Service) View() (service.SettingsView, error) {
	b, err := s.backend()
	if err != nil {
		return service.SettingsView{}, err
	}
	cfg := b.Config.GetConfig()
	theme := cfg.GUI.Theme.Normalized()
	return service.SettingsView{
		Language:           string(i18n.CurrentLang()),
		ThemeBase:          theme.Base,
		ThemeAccent:        theme.Accent,
		UpdateCheckEnabled: cfg.UpdateCheck.Enabled,
	}, nil
}

// Save は Settings を実装する。変更するのは language・gui.theme・update_check.enabled のみ（tui.theme は変えない）。
func (s *Service) Save(in service.SettingsView) (service.FormErrors, error) {
	b, err := s.backend()
	if err != nil {
		return nil, err
	}
	in.Language = strings.TrimSpace(in.Language)
	if errs := validate(in); len(errs) > 0 {
		return errs, nil
	}
	if err := b.Config.UpdateConfig(func(c *core.Config) {
		c.Language = in.Language
		c.GUI.Theme = guiconfig.Theme{Base: in.ThemeBase, Accent: in.ThemeAccent}
		c.UpdateCheck.Enabled = in.UpdateCheckEnabled
	}); err != nil {
		return nil, err
	}
	if err := i18n.SetLang(i18n.Lang(in.Language)); err != nil {
		return nil, err
	}
	if b.Version != nil {
		b.Version.SetEnabled(in.UpdateCheckEnabled)
	}
	return nil, nil
}

func validate(in service.SettingsView) service.FormErrors {
	errs := service.FormErrors{}
	if !supportedLang(in.Language) {
		errs["language"] = i18n.T("gui.settings.error.language")
	}
	if !guiconfig.ValidBase(in.ThemeBase) {
		errs["theme_base"] = i18n.T("gui.settings.error.theme_base")
	}
	if !guiconfig.ValidAccent(in.ThemeAccent) {
		errs["theme_accent"] = i18n.T("gui.settings.error.theme_accent")
	}
	return errs
}

func supportedLang(lang string) bool {
	for _, l := range i18n.SupportedLangs() {
		if string(l.Code) == lang {
			return true
		}
	}
	return false
}

// OpenUpdate は Settings を実装する。
func (s *Service) OpenUpdate() error {
	b, err := s.backend()
	if err != nil {
		return err
	}
	r, ok := service.AvailableUpdate(b)
	if !ok {
		return ErrNoUpdate
	}
	s.open.OpenURL(r.ReleaseURL)
	return nil
}
