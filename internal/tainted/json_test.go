package tainted_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/tainted"
)

// TestMarshalJSONFailsLoudly pins the fix for the silent-drop bug: json.Marshal
// on a struct holding a tainted.String used to produce {} for that field
// (unexported payload, no marshaller) instead of failing. A String exists so
// the compiler forces a sanitizer choice at every render sink; a quiet
// MarshalJSON would open a second unwrap path that bypasses Line and Body, so
// the fix makes it error instead.
func TestMarshalJSONFailsLoudly(t *testing.T) {
	s := struct {
		Message tainted.String
	}{tainted.Wrap(hostile)}

	out, err := json.Marshal(s)
	assert.Error(t, err)
	assert.NotContains(t, string(out), "\x1b]")
	assert.NotContains(t, string(out), "\u202e")
}

// No UnmarshalJSON test: encoding/json already refuses to decode a JSON
// string into tainted.String on its own, since the type has no exported
// string-kind field and no unmarshaller - "cannot unmarshal string into Go
// struct field ... of type tainted.String". Verified directly against the
// unfixed code; a test asserting that error would pass without the fix
// below, so it was not added. The round trip stays symmetric: both
// directions fail loudly without any custom Unmarshal code.
