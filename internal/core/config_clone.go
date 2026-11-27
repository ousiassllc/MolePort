package core

import (
	"slices"
)

// Clone は Config のディープコピーを返す。
// Hosts マップ・ReconnectOverride のポインタ・Forwards スライスを複製し、コピー元と状態を共有しない。
func (c *Config) Clone() *Config {
	out := *c
	out.Forwards = slices.Clone(c.Forwards)
	if c.Hosts != nil {
		out.Hosts = make(map[string]HostConfig, len(c.Hosts))
		for name, hc := range c.Hosts {
			if hc.Reconnect != nil {
				r := *hc.Reconnect
				r.Enabled = clonePtr(r.Enabled)
				r.MaxRetries = clonePtr(r.MaxRetries)
				r.InitialDelay = clonePtr(r.InitialDelay)
				r.MaxDelay = clonePtr(r.MaxDelay)
				hc.Reconnect = &r
			}
			out.Hosts[name] = hc
		}
	}
	return &out
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
