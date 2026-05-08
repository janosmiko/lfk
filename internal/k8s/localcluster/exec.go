package localcluster

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/janosmiko/lfk/internal/logger"
)

// CmdRunner abstracts the two os/exec calls each Provider needs:
// LookPath (for Installed()) and Run (for everything else). Production
// uses realRunner; tests inject FakeRunner for hermetic unit tests.
type CmdRunner interface {
	LookPath(name string) (string, error)
	Run(ctx context.Context, name string, args ...string) (stdout, stderr string, exitCode int, err error)
}

// realRunner is the production CmdRunner. It shells out via os/exec.
type realRunner struct{}

func (realRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (realRunner) Run(ctx context.Context, name string, args ...string) (string, string, int, error) {
	// Log the full argv before execution so the user can see exactly
	// what the local-cluster manager invokes. Visible in the in-app
	// error log (`!`) and at $XDG_DATA_HOME/lfk/lfk.log. Result line
	// follows below so duration + exit code line up with the start.
	logger.Info("localcluster: exec",
		"cmd", name,
		"args", strings.Join(args, " "),
	)
	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	dur := time.Since(start).Round(time.Millisecond)
	code := 0
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	case err != nil:
		// Non-exit failure (context cancel, binary missing, etc.). Wrap
		// with the binary name so callers know which command failed.
		// Don't wrap ExitError — callers type-assert it for stderr access.
		logger.Warn("localcluster: exec failed",
			"cmd", name,
			"args", strings.Join(args, " "),
			"duration", dur.String(),
			"error", err.Error(),
		)
		return so.String(), se.String(), 0, fmt.Errorf("run %q: %w", name, err)
	}
	if code != 0 {
		logger.Warn("localcluster: exec non-zero exit",
			"cmd", name,
			"args", strings.Join(args, " "),
			"duration", dur.String(),
			"exit", code,
			"stderr", strings.TrimSpace(se.String()),
		)
	} else {
		logger.Info("localcluster: exec ok",
			"cmd", name,
			"args", strings.Join(args, " "),
			"duration", dur.String(),
		)
	}
	return so.String(), se.String(), code, err
}

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
