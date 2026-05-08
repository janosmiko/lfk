//go:build !windows

package localcluster

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestRealRunnerLookPathFindsShell(t *testing.T) {
	r := realRunner{}
	path, err := r.LookPath("sh")
	if err != nil {
		t.Fatalf("LookPath sh: %v", err)
	}
	if !strings.HasSuffix(path, "/sh") && path != "sh" {
		t.Fatalf("expected /sh suffix, got %q", path)
	}
}

func TestRealRunnerRun(t *testing.T) {
	tests := []struct {
		name       string
		cancelCtx  bool
		cmdArgs    []string
		wantCode   int
		wantErr    bool
		wantStdout string
		wantStderr string
	}{
		{
			name:       "exit 0 with stdout and stderr",
			cmdArgs:    []string{"-c", "echo hello && echo bye 1>&2 ; exit 0"},
			wantCode:   0,
			wantStdout: "hello",
			wantStderr: "bye",
		},
		{
			name:       "non-zero exit captures code and stderr",
			cmdArgs:    []string{"-c", "echo boom 1>&2 ; exit 7"},
			wantCode:   7,
			wantErr:    true,
			wantStderr: "boom",
		},
		{
			name:      "cancelled context returns wrapped non-ExitError",
			cancelCtx: true,
			cmdArgs:   []string{"-c", "sleep 5"},
			wantCode:  0,
			wantErr:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel() // pre-cancel so the cmd never starts
			}
			r := realRunner{}
			stdout, stderr, code, err := r.Run(ctx, "sh", tc.cmdArgs...)
			if tc.wantErr && err == nil {
				t.Fatalf("expected non-nil err")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if code != tc.wantCode {
				t.Fatalf("code = %d, want %d", code, tc.wantCode)
			}
			if tc.wantStdout != "" && !strings.Contains(stdout, tc.wantStdout) {
				t.Fatalf("stdout = %q, want contains %q", stdout, tc.wantStdout)
			}
			if tc.wantStderr != "" && !strings.Contains(stderr, tc.wantStderr) {
				t.Fatalf("stderr = %q, want contains %q", stderr, tc.wantStderr)
			}
			if tc.cancelCtx {
				// Verify the wrap text is in place: error message must
				// mention the binary name "sh".
				if err != nil && !strings.Contains(err.Error(), `run "sh"`) {
					t.Fatalf("expected wrap with binary name, got %v", err)
				}
			}
		})
	}
}

func TestFakeRunnerSubstitutes(t *testing.T) {
	f := &FakeRunner{
		LookPathFn: func(name string) (string, error) {
			if name == "kind" {
				return "/usr/bin/kind", nil
			}
			return "", exec.ErrNotFound
		},
		RunFn: func(ctx context.Context, name string, args ...string) (string, string, int, error) {
			return "fake-out", "", 0, nil
		},
	}
	if _, err := f.LookPath("k3d"); err == nil {
		t.Fatalf("expected error for k3d")
	}
	out, _, _, err := f.Run(context.Background(), "kind", "version")
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if out != "fake-out" {
		t.Fatalf("Run stdout = %q", out)
	}
	calls := f.CallsSnapshot()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "kind" || len(calls[0].Args) != 1 || calls[0].Args[0] != "version" {
		t.Fatalf("unexpected call: %+v", calls[0])
	}
}
