package k8s

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// markHot runs once per watch tick per pinned view, so it must not walk
// every (context, GVR) entry each time: a GVR already past hotTTL
// survives a markHot landing inside the sweep interval.
func TestMarkHot_DoesNotSweepOnEveryCall(t *testing.T) {
	dc := newFakeDynClient(pod("api-1", "team-a"), namespaceObj("default"))
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAuto)
	t.Cleanup(c.Shutdown)

	ic := c.informers
	gvr := podGVR()
	require.True(t, ic.markHot("", gvr))

	ic.hotTTL = time.Millisecond
	time.Sleep(5 * time.Millisecond)

	nsGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	ic.markHot("", nsGVR)
	assert.True(t, ic.isPromoted("", gvr),
		"a markHot inside the sweep interval must not pay for a sweep")

	ic.sweepInterval = 0
	ic.markHot("", nsGVR)
	assert.False(t, ic.isPromoted("", gvr),
		"the sweep must still run once the interval has elapsed")
}

// getAutoState allocates one entry per GVR ever routed, so entries
// carrying no live decision must not survive a sweep. Promoted and denied
// entries do -- both still drive routing.
func TestSweepStaleHot_PrunesColdAutoState(t *testing.T) {
	ic := newInformerCache(func(string) (dynamic.Interface, error) {
		return nil, errors.New("no dynamic client in this test")
	})

	for i := range 50 {
		cold := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: fmt.Sprintf("widgets%d", i)}
		ic.isPromoted("", cold)
	}

	promoted := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "big"}
	ic.observeDirectListSize("", promoted, ic.promoteAt)

	denied := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "forbidden"}
	state := ic.getAutoState("", denied)
	state.mu.Lock()
	state.denied = true
	state.mu.Unlock()

	require.Len(t, ic.auto[""], 52)

	ic.sweepStaleHot()

	assert.Len(t, ic.auto[""], 2, "cold auto-state entries must not survive a sweep")
	assert.True(t, ic.isPromoted("", promoted))
	assert.True(t, ic.getAutoState("", denied).isDenied())
}
