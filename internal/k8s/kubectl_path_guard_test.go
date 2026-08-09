package k8s

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// bannedKubectlLookup is the exact call every production call site must
// route through KubectlPath instead — see kubectl_path.go. A literal
// substring match is enough here because the banned expression is a fixed
// call with a fixed string-literal argument, not a pattern that a method
// chain or a rename could vary.
const bannedKubectlLookup = `exec.LookPath("kubectl")`

// guardExemptFiles are the only files allowed to contain the banned call:
// the resolver's own implementation, its test's expected-value fixture, and
// this guard file, which necessarily quotes the banned literal to define it.
var guardExemptFiles = map[string]bool{
	"kubectl_path.go":            true,
	"kubectl_path_test.go":       true,
	"kubectl_path_guard_test.go": true,
}

// TestNoBareKubectlLookPath walks the module for the literal
// exec.LookPath("kubectl") call outside the shared resolver. Every other
// call site must go through KubectlPath so demo mode's KUBECTL_BIN override
// (which points kubeconfig precedence at /dev/null) cannot be bypassed by a
// path that falls back to the user's real default kubeconfig.
func TestNoBareKubectlLookPath(t *testing.T) {
	root := moduleRoot(t)

	var offenders []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if guardExemptFiles[filepath.Base(path)] {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), bannedKubectlLookup) {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking module root: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("found bare %s outside the shared resolver; route these through k8s.KubectlPath() instead:\n%s",
			bannedKubectlLookup, strings.Join(offenders, "\n"))
	}
}

// moduleRoot locates the repository root by walking up from this test file
// until a go.mod is found, so the guard scans the whole module regardless of
// the working directory `go test` was invoked from.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to resolve this test file's path")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod above " + thisFile)
		}
		dir = parent
	}
}
