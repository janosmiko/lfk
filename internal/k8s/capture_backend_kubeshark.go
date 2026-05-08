package k8s

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KubesharkInfo describes a deployed kubeshark hub Service.
type KubesharkInfo struct {
	Namespace  string
	HubService string
	HubPort    int32
	Version    string // best-effort from Pod image tag; may be ""
}

// DetectKubeshark returns a *KubesharkInfo if the kubeshark hub Service exists
// in the configured namespace; (nil, nil) if not found; (nil, err) on RBAC
// or other API errors.
func (c *Client) DetectKubeshark(ctx context.Context, kubectx string) (*KubesharkInfo, error) {
	ns := c.kubesharkNamespace()
	cs, err := c.clientsetForContext(kubectx)
	if err != nil {
		return nil, fmt.Errorf("clientset: %w", err)
	}
	svc, err := cs.CoreV1().Services(ns).Get(ctx, "kubeshark-hub", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get kubeshark-hub service: %w", err)
	}
	if len(svc.Spec.Ports) == 0 {
		return nil, fmt.Errorf("kubeshark-hub service has no ports")
	}
	return &KubesharkInfo{
		Namespace:  ns,
		HubService: "kubeshark-hub",
		HubPort:    svc.Spec.Ports[0].Port,
	}, nil
}

// kubesharkNamespace returns the namespace where lfk probes for Service
// kubeshark-hub. Reads c.kubesharkNamespaceOverride (set once at startup
// from the config file via SetKubesharkNamespace) and falls back to
// "kubeshark" — the default of `helm install kubeshark kubeshark/kubeshark
// --namespace kubeshark`.
func (c *Client) kubesharkNamespace() string {
	if c.kubesharkNamespaceOverride != "" {
		return c.kubesharkNamespaceOverride
	}
	return "kubeshark"
}

// LaunchKubeshark starts a port-forward to the kubeshark hub and opens a browser
// pre-filtered to the given pod. The caller supplies the PortForwardManager,
// the kubectlPath (resolved at the app layer via exec.LookPath), and the
// browser-open callable. The port-forward stays alive after this function
// returns so the user can browse the hub UI; users stop it from __port_forwards__.
func (c *Client) LaunchKubeshark(
	ctx context.Context,
	kubectx, _ /*ns*/, pod string, // ns ignored; kubeshark namespace comes from config
	mgr *PortForwardManager,
	kubectlPath string,
	openBrowser func(url string) error,
) error {
	info, err := c.DetectKubeshark(ctx, kubectx)
	if err != nil {
		return fmt.Errorf("detect kubeshark: %w", err)
	}
	if info == nil {
		return fmt.Errorf("kubeshark hub not found")
	}

	kubeconfigPaths := c.KubeconfigPathForContext(kubectx)
	id, err := mgr.Start(kubectlPath, kubeconfigPaths, "svc", info.HubService, info.Namespace,
		kubectx, kubectx, "0", strconv.FormatInt(int64(info.HubPort), 10))
	if err != nil {
		return fmt.Errorf("port-forward start: %w", err)
	}

	// Wait for status to flip to Running, up to 8s. The select honours ctx
	// cancellation so a parent shutdown unblocks the goroutine immediately
	// rather than burning up to 8 seconds on the deadline.
	localPort, err := waitForKubesharkPort(ctx, mgr, id, 8*time.Second, 100*time.Millisecond)
	if err != nil {
		return err
	}
	return openBrowser(kubesharkURL(localPort, pod))
}

// waitForKubesharkPort polls mgr.Entries() for the given port-forward ID to
// reach Running with a non-zero LocalPort. Honours ctx cancellation and the
// total timeout.
//
// LocalPort starts as "0"/empty until the kubectl port-forward subprocess
// resolves the local port; strconv.Atoi error is treated as "not yet ready"
// rather than a hard failure.
func waitForKubesharkPort(ctx context.Context, mgr *PortForwardManager, id int, timeout, tick time.Duration) (int, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	check := func() (int, bool) {
		for _, e := range mgr.Entries() {
			if e.ID != id || e.Status != PortForwardRunning {
				continue
			}
			// LocalPort is "" until the subprocess resolves it; treat parse
			// errors and zero as "not ready yet" rather than terminal.
			p, err := strconv.Atoi(e.LocalPort)
			if err != nil || p == 0 {
				return 0, false
			}
			return p, true
		}
		return 0, false
	}

	if p, ok := check(); ok {
		return p, nil
	}
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-timer.C:
			return 0, fmt.Errorf("kubeshark port-forward didn't reach Running within %s — check __port_forwards__ for status", timeout)
		case <-ticker.C:
			if p, ok := check(); ok {
				return p, nil
			}
		}
	}
}

// kubesharkURL builds the hub URL with a name == "<pod>" KFL filter pre-applied.
func kubesharkURL(localPort int, podName string) string {
	q := fmt.Sprintf(`name == "%s"`, podName)
	return fmt.Sprintf("http://localhost:%d/?q=%s", localPort, url.QueryEscape(q))
}
