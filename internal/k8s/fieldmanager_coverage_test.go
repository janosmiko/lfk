package k8s

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bareOptions are the option literals that reach the apiserver without naming
// a field manager. Every one of them leaves the write attributed to the tool
// instead of the person, which is the whole point of TASK-867.
var bareOptions = []string{
	"metav1.UpdateOptions{}",
	"metav1.PatchOptions{}",
	"metav1.CreateOptions{}",
}

// selfSubjectReviews are authorization queries. They create no stored object,
// so they carry no managedFields and need no field manager.
var selfSubjectReviews = []string{
	"SelfSubjectAccessReviews()",
	"SelfSubjectRulesReviews()",
}

func TestEveryWriteNamesAFieldManager(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)

	var offenders []string
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		offenders = append(offenders, scanBareWrites(path, string(data))...)
	}

	assert.Empty(t, offenders, "these writes reach the apiserver with no field manager")
}

// TestScanBareWrites_FiresOnKnownBadCode proves the scanner above can fail.
// A guard that has never fired on bad input proves nothing.
func TestScanBareWrites_FiresOnKnownBadCode(t *testing.T) {
	bad := "cs.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})"
	assert.Len(t, scanBareWrites("fake.go", bad), 1)

	good := "cs.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{FieldManager: FieldManager()})"
	assert.Empty(t, scanBareWrites("fake.go", good))

	exempt := "clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})"
	assert.Empty(t, scanBareWrites("fake.go", exempt))
}

func scanBareWrites(path, src string) []string {
	var offenders []string
	for i, line := range strings.Split(src, "\n") {
		if isExemptWrite(line) {
			continue
		}
		for _, bare := range bareOptions {
			if strings.Contains(line, bare) {
				offenders = append(offenders, filepath.Base(path)+":"+itoa(i+1)+" "+strings.TrimSpace(line))
			}
		}
	}
	return offenders
}

func isExemptWrite(line string) bool {
	for _, review := range selfSubjectReviews {
		if strings.Contains(line, review) {
			return true
		}
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
