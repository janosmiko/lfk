package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

func TestForegroundRate(t *testing.T) {
	orig := RateLimitForContext
	t.Cleanup(func() { RateLimitForContext = orig })

	t.Run("nil hook falls back to defaults", func(t *testing.T) {
		RateLimitForContext = nil
		qps, burst := foregroundRate("any")
		assert.Equal(t, DefaultClientQPS, qps)
		assert.Equal(t, DefaultClientBurst, burst)
	})

	t.Run("hook value is used", func(t *testing.T) {
		RateLimitForContext = func(ctx string) (float32, int) {
			if ctx == "prod" {
				return 200, 400
			}
			return DefaultClientQPS, DefaultClientBurst
		}
		qps, burst := foregroundRate("prod")
		assert.Equal(t, float32(200), qps)
		assert.Equal(t, 400, burst)
	})

	t.Run("non-positive hook return falls back to defaults", func(t *testing.T) {
		RateLimitForContext = func(string) (float32, int) { return 0, 0 }
		qps, burst := foregroundRate("any")
		assert.Equal(t, DefaultClientQPS, qps)
		assert.Equal(t, DefaultClientBurst, burst)
	})
}

func TestApplyRateLimit(t *testing.T) {
	t.Run("positive values set fields", func(t *testing.T) {
		cfg := &rest.Config{}
		applyRateLimit(cfg, 50, 100)
		assert.Equal(t, float32(50), cfg.QPS)
		assert.Equal(t, 100, cfg.Burst)
	})

	t.Run("non-positive values leave fields untouched", func(t *testing.T) {
		cfg := &rest.Config{QPS: 7, Burst: 9}
		applyRateLimit(cfg, 0, 0)
		assert.Equal(t, float32(7), cfg.QPS, "zero QPS must not clobber existing")
		assert.Equal(t, 9, cfg.Burst, "zero Burst must not clobber existing")
	})
}
