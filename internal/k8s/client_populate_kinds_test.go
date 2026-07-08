package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHumanizeMemory(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"6895736Ki", "6.6Gi"}, // node allocatable as the kubelet reports it
		{"128Mi", "128Mi"},
		{"1Gi", "1Gi"}, // whole values drop the .0
		{"8Gi", "8Gi"},
		{"1024Ki", "1Mi"},
		{"512Ki", "512Ki"},
		{"512", "512B"},
		{"not-a-quantity", "not-a-quantity"}, // unparseable falls back unchanged
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, humanizeMemory(tt.in))
		})
	}
}
