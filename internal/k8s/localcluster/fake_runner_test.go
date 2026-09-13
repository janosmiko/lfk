package localcluster

import (
	"context"
	"os/exec"
	"sync"
)

// FakeRunner is a CmdRunner whose behavior is fully driven by the test
// (no fixtures, no shared state). Tests set LookPathFn / RunFn per case.
type FakeRunner struct {
	LookPathFn func(name string) (string, error)
	RunFn      func(ctx context.Context, name string, args ...string) (stdout, stderr string, exitCode int, err error)

	mu    sync.Mutex
	Calls []FakeCall
}

// FakeCall captures one invocation of FakeRunner.Run.
type FakeCall struct {
	Name string
	Args []string
}

func (f *FakeRunner) LookPath(name string) (string, error) {
	if f.LookPathFn == nil {
		return "", exec.ErrNotFound
	}
	return f.LookPathFn(name)
}

func (f *FakeRunner) Run(ctx context.Context, name string, args ...string) (string, string, int, error) {
	f.mu.Lock()
	f.Calls = append(f.Calls, FakeCall{Name: name, Args: append([]string(nil), args...)})
	f.mu.Unlock()
	if f.RunFn == nil {
		return "", "", 0, nil
	}
	return f.RunFn(ctx, name, args...)
}

// CallsSnapshot returns a copy of the recorded calls. Tests should use
// this instead of reading .Calls directly when assertions might race
// with in-flight Run() invocations.
func (f *FakeRunner) CallsSnapshot() []FakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]FakeCall, len(f.Calls))
	copy(out, f.Calls)
	return out
}
