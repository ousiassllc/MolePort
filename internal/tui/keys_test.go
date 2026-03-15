package tui

import (
	"slices"
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestDefaultKeyMap_AllBindingsHaveKeys(t *testing.T) {
	km := DefaultKeyMap()

	bindings := []struct {
		name    string
		binding key.Binding
	}{
		{"Tab", km.Tab},
		{"Help", km.Help},
		{"Search", km.Search},
		{"Escape", km.Escape},
		{"Quit", km.Quit},
		{"ForceQuit", km.ForceQuit},
		{"Up", km.Up},
		{"Down", km.Down},
		{"Enter", km.Enter},
		{"Disconnect", km.Disconnect},
		{"Delete", km.Delete},
		{"Theme", km.Theme},
		{"Lang", km.Lang},
		{"Version", km.Version},
	}

	for _, b := range bindings {
		t.Run(b.name, func(t *testing.T) {
			keys := b.binding.Keys()
			if len(keys) == 0 {
				t.Errorf("binding %s has no keys", b.name)
			}
			help := b.binding.Help()
			if help.Key == "" {
				t.Errorf("binding %s has empty help key", b.name)
			}
			if help.Desc == "" {
				t.Errorf("binding %s has empty help description", b.name)
			}
		})
	}
}

func TestDefaultKeyMap_ShortHelp(t *testing.T) {
	km := DefaultKeyMap()
	bindings := km.ShortHelp()

	if len(bindings) == 0 {
		t.Fatal("ShortHelp should return at least one binding")
	}
	if len(bindings) != 3 {
		t.Errorf("ShortHelp should return 3 bindings (Tab, Help, Quit), got %d", len(bindings))
	}
}

func TestDefaultKeyMap_FullHelp(t *testing.T) {
	km := DefaultKeyMap()
	groups := km.FullHelp()

	if len(groups) != 3 {
		t.Fatalf("FullHelp should return 3 groups, got %d", len(groups))
	}

	// グループ1: グローバルキー (Tab, Help, Search, Escape, Quit, ForceQuit)
	if len(groups[0]) != 6 {
		t.Errorf("group 0 should have 6 bindings, got %d", len(groups[0]))
	}

	// グループ2: ナビゲーション (Up, Down)
	if len(groups[1]) != 2 {
		t.Errorf("group 1 should have 2 bindings, got %d", len(groups[1]))
	}

	// グループ3: アクション (Enter, Disconnect, Delete, Theme, Lang, Version)
	if len(groups[2]) != 6 {
		t.Errorf("group 2 should have 6 bindings, got %d", len(groups[2]))
	}
}

func TestDefaultKeyMap_SpecificKeys(t *testing.T) {
	km := DefaultKeyMap()

	tests := []struct {
		name    string
		binding key.Binding
		wantKey string
	}{
		{"Tab", km.Tab, "tab"},
		{"Help", km.Help, "?"},
		{"Search", km.Search, "/"},
		{"Escape", km.Escape, "esc"},
		{"Quit", km.Quit, "q"},
		{"Enter", km.Enter, "enter"},
		{"Disconnect", km.Disconnect, "d"},
		{"Delete", km.Delete, "x"},
		{"Theme", km.Theme, "t"},
		{"Lang", km.Lang, "l"},
		{"Version", km.Version, "v"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := tt.binding.Keys()
			if !slices.Contains(keys, tt.wantKey) {
				t.Errorf("binding %s should contain key %q, got %v", tt.name, tt.wantKey, keys)
			}
		})
	}
}

func TestDefaultKeyMap_UpDown_HaveMultipleKeys(t *testing.T) {
	km := DefaultKeyMap()

	// Up は "up" と "k" の両方
	upKeys := km.Up.Keys()
	if len(upKeys) < 2 {
		t.Errorf("Up should have at least 2 keys (up, k), got %v", upKeys)
	}

	// Down は "down" と "j" の両方
	downKeys := km.Down.Keys()
	if len(downKeys) < 2 {
		t.Errorf("Down should have at least 2 keys (down, j), got %v", downKeys)
	}
}
