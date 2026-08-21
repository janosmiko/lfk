package k8s

// updateListener is the single-slot registry the managers use to push state
// changes at the UI. It holds no lock of its own: the owner calls every method
// under the mutex guarding the state being announced.
type updateListener struct {
	ch         chan<- struct{}
	superseded chan struct{}
}

// setLocked installs ch as the only listener. The returned channel closes when
// a later call retires it, so the waiter on the old channel stops waiting
// instead of leaking.
func (l *updateListener) setLocked(ch chan<- struct{}) <-chan struct{} {
	if l.superseded != nil {
		close(l.superseded)
	}
	l.ch = ch
	l.superseded = make(chan struct{})
	return l.superseded
}

// notifyLocked delivers one update. The send is non-blocking: a listener that
// already returned must never stall the manager.
func (l *updateListener) notifyLocked() {
	if l.ch == nil {
		return
	}
	select {
	case l.ch <- struct{}{}:
	default:
	}
}
