package k8s

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
)

func TestExtractImageFromMessage(t *testing.T) {
	tests := []struct {
		message  string
		expected string
	}{
		{`Pulling image "nginx:latest"`, "nginx:latest"},
		{`Successfully pulled image "redis:7" in 2.5s`, "redis:7"},
		{`No quotes here`, ""},
		{`One "quote only`, ""},
		{``, ""},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			result := extractImageFromMessage(tt.message)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestComputeImagePullTime(t *testing.T) {
	t.Run("paired pulling and pulled events", func(t *testing.T) {
		now := time.Now()
		events := []corev1.Event{
			{
				Reason:  "Pulling",
				Message: `Pulling image "nginx:latest"`,
				LastTimestamp: metav1.Time{Time: now},
			},
			{
				Reason:  "Pulled",
				Message: `Successfully pulled image "nginx:latest" in 3s`,
				LastTimestamp: metav1.Time{Time: now.Add(3 * time.Second)},
			},
		}
		result := computeImagePullTime(events)
		assert.Equal(t, 3*time.Second, result)
	})

	t.Run("multiple images", func(t *testing.T) {
		now := time.Now()
		events := []corev1.Event{
			{
				Reason:  "Pulling",
				Message: `Pulling image "nginx:latest"`,
				LastTimestamp: metav1.Time{Time: now},
			},
			{
				Reason:  "Pulled",
				Message: `Successfully pulled image "nginx:latest"`,
				LastTimestamp: metav1.Time{Time: now.Add(2 * time.Second)},
			},
			{
				Reason:  "Pulling",
				Message: `Pulling image "redis:7"`,
				LastTimestamp: metav1.Time{Time: now.Add(2 * time.Second)},
			},
			{
				Reason:  "Pulled",
				Message: `Successfully pulled image "redis:7"`,
				LastTimestamp: metav1.Time{Time: now.Add(5 * time.Second)},
			},
		}
		result := computeImagePullTime(events)
		assert.Equal(t, 5*time.Second, result) // 2s + 3s
	})

	t.Run("no events returns zero", func(t *testing.T) {
		result := computeImagePullTime(nil)
		assert.Equal(t, time.Duration(0), result)
	})

	t.Run("pulling without pulled returns zero", func(t *testing.T) {
		now := time.Now()
		events := []corev1.Event{
			{
				Reason:  "Pulling",
				Message: `Pulling image "nginx:latest"`,
				LastTimestamp: metav1.Time{Time: now},
			},
		}
		result := computeImagePullTime(events)
		assert.Equal(t, time.Duration(0), result)
	})

	t.Run("non-pull events ignored", func(t *testing.T) {
		now := time.Now()
		events := []corev1.Event{
			{
				Reason:  "Scheduled",
				Message: "Successfully assigned pod",
				LastTimestamp: metav1.Time{Time: now},
			},
			{
				Reason:  "Created",
				Message: "Created container",
				LastTimestamp: metav1.Time{Time: now.Add(1 * time.Second)},
			},
		}
		result := computeImagePullTime(events)
		assert.Equal(t, time.Duration(0), result)
	})
}
