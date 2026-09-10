package app

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// metricsSeriesCache mutates its maps in place. That is only safe while no
// retained copy of the Model can observe the write, which holds as long as the
// cache is not carried by TabState. Adding it there means the maps must become
// copy on write first.
func TestMetricsSeriesCacheIsNotSnapshottedPerTab(t *testing.T) {
	cacheType := reflect.TypeFor[metricsSeriesCache]()
	for f := range reflect.TypeFor[TabState]().Fields() {
		held := f.Type == cacheType || f.Type == reflect.PointerTo(cacheType)
		assert.False(t, held,
			"TabState.%s snapshots the series cache; make its maps copy on write before keeping this", f.Name)
	}
}
