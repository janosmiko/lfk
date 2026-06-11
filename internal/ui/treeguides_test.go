package ui

import (
	"reflect"
	"testing"
)

func TestTreeGuidePrefixes(t *testing.T) {
	tests := []struct {
		name   string
		depths []int
		want   []string
	}{
		{
			name:   "empty",
			depths: nil,
			want:   []string{},
		},
		{
			name:   "flat siblings",
			depths: []int{0, 0, 0},
			want:   []string{"├─ ", "├─ ", "└─ "},
		},
		{
			name:   "nested with continuation stem",
			depths: []int{0, 1, 1, 0},
			want:   []string{"├─ ", "│  ├─ ", "│  └─ ", "└─ "},
		},
		{
			name:   "last parent has blank stem",
			depths: []int{0, 0, 1, 2, 1},
			want:   []string{"├─ ", "└─ ", "   ├─ ", "   │  └─ ", "   └─ "},
		},
		{
			name:   "single chain",
			depths: []int{0, 1, 2},
			want:   []string{"└─ ", "   └─ ", "      └─ "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TreeGuidePrefixes(tt.depths)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TreeGuidePrefixes(%v)\n got: %q\nwant: %q", tt.depths, got, tt.want)
			}
		})
	}
}
