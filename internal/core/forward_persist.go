package core

import "fmt"

// PersistForwardRules は ForwardManager が保持する現在のフォワードルールを設定ファイルに保存する。
// IPC ハンドラと GUI Service の両方から利用する（ルール保存の二重実装を避ける）。
func PersistForwardRules(cfg ConfigManager, fwd ForwardManager) error {
	// スナップショットは設定ロック内で取得する。
	// ロック外で取ると、古い一覧を持つ呼び出しが後から書き込んで新しいルールを失う。
	// ForwardManager は ConfigManager のロックを取らないためデッドロックしない。
	if err := cfg.UpdateConfig(func(c *Config) {
		c.Forwards = fwd.GetRules()
	}); err != nil {
		return fmt.Errorf("persist forward rules: %w", err)
	}
	return nil
}
