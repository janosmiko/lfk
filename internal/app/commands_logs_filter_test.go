package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsKubectlTransientError(t *testing.T) {
	cases := []struct {
		name    string
		line    string
		want    bool
		comment string
	}{
		{
			name:    "PodInitializing noise",
			line:    `error: container "main" in pod "demo-pod" is waiting to start: PodInitializing`,
			want:    true,
			comment: "kubectl spam while an init container blocks main from starting",
		},
		{
			name:    "ContainerCreating noise",
			line:    `error: container "init" in pod "demo-pod" is waiting to start: ContainerCreating`,
			want:    true,
			comment: "kubectl spam before the container has been scheduled",
		},
		{
			name: "BadRequest not valid for pod",
			line: `Error from server (BadRequest): container "main" in pod "demo-pod" is not valid for pod "demo-pod"`,
			want: true,
		},
		{
			name:    "real log line passes through",
			line:    `[pod/demo-pod/init] 2026-04-20T14:20:39.474690359Z [init] step 5/10`,
			want:    false,
			comment: "legitimate prefixed log lines must not be filtered",
		},
		{
			name:    "timestamp-only line passes through",
			line:    `2026-04-20T14:20:39.474690359Z [init] step 5/10`,
			want:    false,
			comment: "non-prefixed app output must not be filtered",
		},
		{
			name:    "generic error passes through",
			line:    `error: something else went wrong`,
			want:    false,
			comment: "only the specific 'waiting to start' pattern is noise",
		},
		{
			name:    "empty line passes through",
			line:    "",
			want:    false,
			comment: "trivial safety — the filter never panics on empty input",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isKubectlTransientError(tc.line)
			assert.Equal(t, tc.want, got, tc.comment)
		})
	}
}
