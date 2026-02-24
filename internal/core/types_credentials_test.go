package core

import "testing"

func TestCredentialType_Constants(t *testing.T) {
	tests := []struct {
		ct   CredentialType
		want string
	}{
		{CredentialPassword, "password"},
		{CredentialPassphrase, "passphrase"},
		{CredentialKeyboardInteractive, "keyboard-interactive"},
	}
	for _, tt := range tests {
		if got := string(tt.ct); got != tt.want {
			t.Errorf("CredentialType = %q, want %q", got, tt.want)
		}
	}
}
