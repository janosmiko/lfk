package model

import "testing"

func TestIconIsEmpty(t *testing.T) {
	tests := []struct {
		name string
		icon Icon
		want bool
	}{
		{"zero value is empty", Icon{}, true},
		{"unicode-only not empty", Icon{Unicode: "X"}, false},
		{"simple-only not empty", Icon{Simple: "[Po]"}, false},
		{"emoji-only not empty", Icon{Emoji: "🔵"}, false},
		{"nerdfont-only not empty", Icon{NerdFont: "\U000f01a7"}, false},
		{"all fields set not empty", Icon{Unicode: "□", Simple: "[Po]", Emoji: "🔵", NerdFont: "\U000f01a7"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.icon.IsEmpty(); got != tc.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tc.want)
			}
		})
	}
}
