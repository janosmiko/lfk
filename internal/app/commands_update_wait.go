package app

import "time"

// waitForUpdateSignal blocks until the manager reports a change on ch, the
// optional deadline fires, or a newer listener supersedes this one. It reports
// false only when the caller was superseded with no update pending.
func waitForUpdateSignal(ch <-chan struct{}, deadlineC <-chan time.Time, superseded <-chan struct{}) bool {
	select {
	case <-ch:
	case <-deadlineC:
	case <-superseded:
		// select can pick superseded over a racing write to ch. Drain once
		// more so that pending update is not dropped.
		select {
		case <-ch:
		default:
			return false
		}
	}
	return true
}
