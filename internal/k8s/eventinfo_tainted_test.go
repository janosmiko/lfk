package k8s_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/tainted"
)

// TestEventInfoHasNoRawStringFields fails when any text field on EventInfo is
// a plain string. Every one of them is cluster-controlled - any principal that
// can create an Event in a watched namespace sets Type, Reason, Message and
// Source - so a plain string here is a sink that a reviewer has to remember to
// sanitize. This is the guard that catches a NEW field, which is how the leak
// recurred: sibling fields in one Sprintf, three of them sanitized.
func TestEventInfoHasNoRawStringFields(t *testing.T) {
	taintedType := reflect.TypeFor[tainted.String]()

	for f := range reflect.TypeFor[k8s.EventInfo]().Fields() {
		assert.NotEqual(t, reflect.String, f.Type.Kind(),
			"EventInfo.%s is a raw string; cluster-controlled text must be %s", f.Name, taintedType)
	}
}
