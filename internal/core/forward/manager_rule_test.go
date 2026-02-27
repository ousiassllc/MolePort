package forward

import (
	"sync"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestForwardManager_GetRules_Order(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	names := []string{"alpha", "beta", "gamma"}
	for _, name := range names {
		if _, err := fm.AddRule(core.ForwardRule{
			Name: name, Host: "server1", Type: core.Dynamic, LocalPort: 1080,
		}); err != nil {
			t.Fatalf("AddRule(%q) error = %v", name, err)
		}
	}

	rules := fm.GetRules()
	if len(rules) != 3 {
		t.Fatalf("len(rules) = %d, want 3", len(rules))
	}
	for i, name := range names {
		if rules[i].Name != name {
			t.Errorf("rules[%d].Name = %q, want %q", i, rules[i].Name, name)
		}
	}
}

func TestForwardManager_GetRulesByHost(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	_, _ = fm.AddRule(core.ForwardRule{Name: "web1", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
	_, _ = fm.AddRule(core.ForwardRule{Name: "web2", Host: "server2", Type: core.Dynamic, LocalPort: 1081})
	_, _ = fm.AddRule(core.ForwardRule{Name: "web3", Host: "server1", Type: core.Dynamic, LocalPort: 1082})

	rules := fm.GetRulesByHost("server1")
	if len(rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(rules))
	}
	if rules[0].Name != "web1" {
		t.Errorf("rules[0].Name = %q, want %q", rules[0].Name, "web1")
	}
	if rules[1].Name != "web3" {
		t.Errorf("rules[1].Name = %q, want %q", rules[1].Name, "web3")
	}
}

func TestForwardManager_GetRulesByHost_Empty(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	rules := fm.GetRulesByHost("nonexistent")
	if len(rules) != 0 {
		t.Errorf("len(rules) = %d, want 0", len(rules))
	}
}

func TestForwardManager_DeleteRule_Concurrent(t *testing.T) {
	sm := newMockSSHManager()
	sm.setConnected("server1", newMockDynamicDefaultConn())
	fm := NewForwardManager(sm)

	_, _ = fm.AddRule(core.ForwardRule{Name: "web", Host: "server1", Type: core.Dynamic, LocalPort: 1080})
	_ = fm.StartForward("web", nil)

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_ = fm.DeleteRule("web")
		}()
	}
	wg.Wait()

	rules := fm.GetRules()
	if len(rules) != 0 {
		t.Errorf("len(rules) = %d, want 0 after concurrent delete", len(rules))
	}
}

func TestForwardManager_AddRule_DefaultRemoteHost(t *testing.T) {
	sm := newMockSSHManager()
	fm := NewForwardManager(sm)

	// Local タイプで RemoteHost を指定しない場合、"localhost" がデフォルトになる
	_, err := fm.AddRule(core.ForwardRule{
		Name:       "web-local",
		Host:       "server1",
		Type:       core.Local,
		LocalPort:  8080,
		RemotePort: 80,
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	rules := fm.GetRules()
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
	if rules[0].RemoteHost != "localhost" {
		t.Errorf("RemoteHost = %q, want %q", rules[0].RemoteHost, "localhost")
	}

	// Remote タイプでも同様
	_, err = fm.AddRule(core.ForwardRule{
		Name:       "web-remote",
		Host:       "server1",
		Type:       core.Remote,
		LocalPort:  3000,
		RemotePort: 80,
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	rules = fm.GetRules()
	if rules[1].RemoteHost != "localhost" {
		t.Errorf("RemoteHost = %q, want %q", rules[1].RemoteHost, "localhost")
	}

	// Dynamic タイプでは RemoteHost はそのまま空
	_, err = fm.AddRule(core.ForwardRule{
		Name:      "socks",
		Host:      "server1",
		Type:      core.Dynamic,
		LocalPort: 1080,
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}

	rules = fm.GetRules()
	if rules[2].RemoteHost != "" {
		t.Errorf("Dynamic RemoteHost = %q, want empty", rules[2].RemoteHost)
	}
}
