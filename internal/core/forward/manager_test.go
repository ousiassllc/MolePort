package forward

import (
	"context"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestForwardManager_AddRule(t *testing.T) {
	fm := NewForwardManager(context.Background(), newMockSSHManager())
	name, err := fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}
	if name != "web" {
		t.Errorf("AddRule() name = %q, want %q", name, "web")
	}
	rules := fm.GetRules()
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
	if rules[0].Name != "web" {
		t.Errorf("rule name = %q, want %q", rules[0].Name, "web")
	}
	if rules[0].Host != "server1" {
		t.Errorf("rule host = %q, want %q", rules[0].Host, "server1")
	}
}

func TestForwardManager_AddRule_AutoName(t *testing.T) {
	fm := NewForwardManager(context.Background(), newMockSSHManager())
	name, err := fm.AddRule(core.ForwardRule{
		Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	})
	if err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}
	if name == "" {
		t.Error("auto-generated name should not be empty")
	}
	rules := fm.GetRules()
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
	if rules[0].Name != name {
		t.Errorf("rule name = %q, want %q", rules[0].Name, name)
	}
}

func TestForwardManager_AddRule_DuplicateName(t *testing.T) {
	fm := NewForwardManager(context.Background(), newMockSSHManager())
	rule := core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	}
	if _, err := fm.AddRule(rule); err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}
	_, err := fm.AddRule(rule)
	if err == nil {
		t.Fatal("AddRule() should return error for duplicate name")
	}
}

func TestForwardManager_AddRule_Validation(t *testing.T) {
	fm := NewForwardManager(context.Background(), newMockSSHManager())
	tests := []struct {
		name    string
		rule    core.ForwardRule
		wantErr bool
	}{
		{"empty host", core.ForwardRule{Name: "t1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80}, true},
		{"zero local port", core.ForwardRule{Name: "t2", Host: "server1", Type: core.Local, LocalPort: 0, RemoteHost: "localhost", RemotePort: 80}, true},
		{"negative local port", core.ForwardRule{Name: "t3", Host: "server1", Type: core.Local, LocalPort: -1, RemoteHost: "localhost", RemotePort: 80}, true},
		{"too large local port", core.ForwardRule{Name: "t4", Host: "server1", Type: core.Local, LocalPort: 65536, RemoteHost: "localhost", RemotePort: 80}, true},
		{"valid min local port", core.ForwardRule{Name: "t5", Host: "server1", Type: core.Local, LocalPort: 1, RemoteHost: "localhost", RemotePort: 80}, false},
		{"valid max local port", core.ForwardRule{Name: "t6", Host: "server1", Type: core.Local, LocalPort: 65535, RemoteHost: "localhost", RemotePort: 80}, false},
		{"valid mid local port", core.ForwardRule{Name: "t7", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80}, false},
		{"invalid remote port", core.ForwardRule{Name: "t8", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 0}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fm.AddRule(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestForwardManager_AddRule_DynamicNoRemotePort(t *testing.T) {
	// Dynamic では RemotePort は不要
	if _, err := NewForwardManager(context.Background(), newMockSSHManager()).AddRule(core.ForwardRule{Name: "socks", Host: "server1", Type: core.Dynamic, LocalPort: 1080}); err != nil {
		t.Fatalf("AddRule() error = %v (Dynamic should not require remote port)", err)
	}
}

func TestForwardManager_DeleteRule(t *testing.T) {
	fm := NewForwardManager(context.Background(), newMockSSHManager())
	if _, err := fm.AddRule(core.ForwardRule{
		Name: "web", Host: "server1", Type: core.Local, LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
	}); err != nil {
		t.Fatalf("AddRule() error = %v", err)
	}
	if err := fm.DeleteRule("web"); err != nil {
		t.Fatalf("DeleteRule() error = %v", err)
	}
	if rules := fm.GetRules(); len(rules) != 0 {
		t.Errorf("len(rules) = %d, want 0", len(rules))
	}
}

func TestForwardManager_DeleteRule_NotFound(t *testing.T) {
	if err := NewForwardManager(context.Background(), newMockSSHManager()).DeleteRule("nonexistent"); err == nil {
		t.Fatal("DeleteRule() should return error for nonexistent rule")
	}
}
