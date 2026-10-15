package shell

import "testing"

func TestIsAffirmative(t *testing.T) {
	tests := []struct {
		name, result, yes string
		want              bool
	}{
		{"ローカライズ済みラベル", "はい", "はい", true},
		{"Windows の固定値", "Yes", "はい", true},
		{"小文字・空白", " yes ", "Yes", true},
		{"否定ラベル", "いいえ", "はい", false},
		{"Windows の No", "No", "はい", false},
		{"キャンセル・空", "", "はい", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAffirmative(tt.result, tt.yes); got != tt.want {
				t.Errorf("IsAffirmative(%q, %q) = %v, want %v", tt.result, tt.yes, got, tt.want)
			}
		})
	}
}
