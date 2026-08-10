package k8s

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DescribePod runs `kubectl describe pod <podName> -n <namespace> --context
// <contextName>` and returns the combined output. Used by
// GetCrashInvestigation to fill the Describe tab when no test override is
// set.
//
// kubectl is required on PATH (lfk already requires it for other commands).
// On non-zero exit the error includes the trimmed stderr/stdout for context.
func (c *Client) DescribePod(ctx context.Context, contextName, namespace, podName string) (string, error) {
	kubectlPath, err := KubectlPath()
	if err != nil {
		return "", fmt.Errorf("kubectl not found: %w", err)
	}

	args := []string{"describe", "pod", podName, "-n", namespace, "--context", contextName}
	cmd := exec.CommandContext(ctx, kubectlPath, DemoKubectlArgs(args)...)
	if path := c.KubeconfigPathForContext(contextName); path != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+path)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
