package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaintString(t *testing.T) {
	assert.Equal(t, "dedicated=gpu:NoSchedule",
		Taint{Key: "dedicated", Value: "gpu", Effect: "NoSchedule"}.String())
	assert.Equal(t, "node.kubernetes.io/unreachable:NoExecute",
		Taint{Key: "node.kubernetes.io/unreachable", Effect: "NoExecute"}.String())
}

func TestValidateTaint(t *testing.T) {
	tests := []struct {
		name    string
		taint   Taint
		wantErr string
	}{
		{"valid bare key", Taint{Key: "dedicated", Effect: "NoSchedule"}, ""},
		{"valid with value", Taint{Key: "dedicated", Value: "gpu", Effect: "NoExecute"}, ""},
		{"valid prefixed key", Taint{Key: "node.kubernetes.io/unreachable", Effect: "PreferNoSchedule"}, ""},
		{"empty key", Taint{Effect: "NoSchedule"}, "key is required"},
		{"bad effect", Taint{Key: "a", Effect: "NoSched"}, "effect"},
		{"empty effect", Taint{Key: "a"}, "effect"},
		{"key with spaces", Taint{Key: "bad key", Effect: "NoSchedule"}, "key"},
		{"key ends with dash", Taint{Key: "bad-", Effect: "NoSchedule"}, "key"},
		{"two slashes", Taint{Key: "a/b/c", Effect: "NoSchedule"}, "key"},
		{"name too long", Taint{Key: strings.Repeat("a", 64), Effect: "NoSchedule"}, "key"},
		{"prefix too long", Taint{Key: strings.Repeat("a", 254) + "/x", Effect: "NoSchedule"}, "key"},
		{"bad value char", Taint{Key: "a", Value: "g pu", Effect: "NoSchedule"}, "value"},
		{"value too long", Taint{Key: "a", Value: strings.Repeat("v", 64), Effect: "NoSchedule"}, "value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTaint(tt.taint)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestComputeFinalTaints(t *testing.T) {
	existing := []Taint{
		{Key: "dedicated", Value: "gpu", Effect: "NoSchedule"},
		{Key: "maintenance", Effect: "NoExecute"},
	}

	t.Run("removal matches by key+effect identity", func(t *testing.T) {
		final := ComputeFinalTaints(existing,
			[]Taint{{Key: "dedicated", Value: "stale-value", Effect: "NoSchedule"}}, nil)
		assert.Equal(t, []Taint{{Key: "maintenance", Effect: "NoExecute"}}, final)
	})

	t.Run("removal of vanished taint is a no-op", func(t *testing.T) {
		final := ComputeFinalTaints(existing,
			[]Taint{{Key: "gone", Effect: "NoSchedule"}}, nil)
		assert.Equal(t, existing, final)
	})

	t.Run("additions append", func(t *testing.T) {
		add := Taint{Key: "team", Value: "ml", Effect: "PreferNoSchedule"}
		final := ComputeFinalTaints(existing, nil, []Taint{add})
		assert.Equal(t, append(append([]Taint{}, existing...), add), final)
	})

	t.Run("duplicate addition is dropped", func(t *testing.T) {
		final := ComputeFinalTaints(existing, nil,
			[]Taint{{Key: "dedicated", Value: "other", Effect: "NoSchedule"}})
		assert.Equal(t, existing, final, "key+effect already present — server would reject")
	})

	t.Run("concurrent add survives a removal pass", func(t *testing.T) {
		// existing includes a taint the editor never saw; removing another
		// taint must not disturb it.
		withConcurrent := append(append([]Taint{}, existing...),
			Taint{Key: "added-elsewhere", Effect: "NoSchedule"})
		final := ComputeFinalTaints(withConcurrent,
			[]Taint{{Key: "maintenance", Effect: "NoExecute"}}, nil)
		assert.Equal(t, []Taint{
			{Key: "dedicated", Value: "gpu", Effect: "NoSchedule"},
			{Key: "added-elsewhere", Effect: "NoSchedule"},
		}, final)
	})

	t.Run("does not mutate inputs", func(t *testing.T) {
		before := append([]Taint{}, existing...)
		_ = ComputeFinalTaints(existing, []Taint{existing[0]}, []Taint{{Key: "x", Effect: "NoExecute"}})
		assert.Equal(t, before, existing)
	})
}
