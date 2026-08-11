package tainted

import "errors"

// errMarshalJSON is returned by MarshalJSON. Without it, json.Marshal on a
// struct holding a String silently produces {} for that field - the payload
// is unexported and the type has no marshaller - so the field vanishes
// instead of erroring. A quiet MarshalJSON that emitted Line() instead would
// fix the vanishing act but reopen the problem the type exists to close: it
// would let any caller serialize cluster text without choosing between Line
// and Body, bypassing both unwraps the same way a bare String() method
// would. Failing loudly keeps that choice mandatory.
var errMarshalJSON = errors.New("tainted.String cannot be marshalled directly; unwrap with Line or Body first")

// MarshalJSON always fails; see errMarshalJSON.
func (t String) MarshalJSON() ([]byte, error) {
	return nil, errMarshalJSON
}
