package core_test

import (
	"errors"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestIsAuthFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated error", errors.New("connection refused"), false},
		{"unable to authenticate", errors.New("unable to authenticate"), true},
		{"no authentication methods available", errors.New("ssh: no authentication methods available"), true},
		{"no supported methods remain", errors.New("no supported methods remain"), true},
		{"wrapped auth error", errors.New("failed to connect: unable to authenticate, giving up"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := core.IsAuthFailure(tt.err); got != tt.want {
				t.Errorf("IsAuthFailure(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
