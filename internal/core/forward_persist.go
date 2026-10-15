package core

import "fmt"

// PersistForwardRules は ForwardManager が保持する現在のフォワードルールを設定ファイルに保存する。
// IPC ハンドラと GUI Service の両方から利用する（ルール保存の二重実装を避ける）。
func PersistForwardRules(cfg ConfigManager, fwd ForwardManager) error {
	rules := fwd.GetRules()
	if err := cfg.UpdateConfig(func(c *Config) {
		c.Forwards = rules
	}); err != nil {
		return fmt.Errorf("persist forward rules: %w", err)
	}
	return nil
}
