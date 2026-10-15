package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ousiassllc/moleport/internal/core"
)

func TestLocalizeError(t *testing.T) {
	useEnglish(t)
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"timeout", core.ErrCredentialTimeout, "Credential input timed out."},
		{"cancelled", fmt.Errorf("wrap: %w", core.ErrCredentialCancelled), "Credential input was cancelled."},
		{"not found", &core.NotFoundError{Resource: "rule", Name: "web"}, `"web" was not found.`},
		{"exists", &core.AlreadyExistsError{Resource: "rule", Name: "web"}, `"web" already exists.`},
		{"active", &core.AlreadyActiveError{Name: "web"}, `"web" is already active.`},
		{"not connected", &core.NotConnectedError{HostName: "h"}, `Host "h" is not connected.`},
		{"auth required", &core.AuthRequiredError{HostName: "h", Err: errors.New("x")}, "Authentication failed."},
		{"auth failure text", errors.New("ssh: unable to authenticate"), "Authentication failed."},
		{"port in use", errors.New("listen tcp :80: bind: address already in use"), "The port is already in use."},
		{"unknown", errors.New("boom"), "boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LocalizeError(tt.err); got != tt.want {
				t.Errorf("LocalizeError() = %q, want %q", got, tt.want)
			}
		})
	}
}
