package k8s

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// kubectlBinForDescribe returns the kubectl binary path used by DescribePod.
// Delegates to KubectlPath; falls back to the bare "kubectl" name (letting
// exec.CommandContext resolve it on PATH at fork time) when resolution
// fails, matching this function's original never-erroring contract.
func kubectlBinForDescribe() string {
	if path, err := KubectlPath(); err == nil {
		return path
	}
	return "kubectl"
}

// DescribePod runs `kubectl describe pod <podName> -n <namespace> --context
// <contextName>` and returns the combined output. Used by
// GetCrashInvestigation to fill the Describe tab when no test override is
// set.
//
// kubectl is required on PATH (lfk already requires it for other commands).
// On non-zero exit the error includes the trimmed stderr/stdout for context.
func (c *Client) DescribePod(ctx context.Context, contextName, namespace, podName string) (string, error) {
	args := []string{"describe", "pod", podName, "-n", namespace, "--context", contextName}
	cmd := exec.CommandContext(ctx, kubectlBinForDescribe(), args...)
	if path := c.KubeconfigPathForContext(contextName); path != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+path)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
