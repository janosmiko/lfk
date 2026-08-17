package tainted

import "errors"

// errMarshalJSON is returned by MarshalJSON. Without it, json.Marshal on a
// struct holding a String silently produces {}. The payload is unexported
// and the type has no marshaller, so the field vanishes instead of erroring.
// A quiet MarshalJSON emitting Line() would fix that, but it would reopen
// the problem the type exists to close. Callers could serialize cluster
// text without choosing between Line and Body, the same bypass a bare
// String() method allows. Failing loudly keeps that choice mandatory.
var errMarshalJSON = errors.New("tainted.String cannot be marshalled directly; unwrap with Line or Body first")

// MarshalJSON always fails. See errMarshalJSON for the reason.
func (t String) MarshalJSON() ([]byte, error) {
	return nil, errMarshalJSON
}
