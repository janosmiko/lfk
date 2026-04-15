package app

// StartupOptions holds command-line flag values that override default startup behavior.
type StartupOptions struct {
	Context    string
	Namespaces []string
	Kubeconfig string
	Config     string
	NoMouse    bool
}

// HasCLIOverrides returns true when any CLI flag was provided.
// Kubeconfig is intentionally excluded: it affects client construction,
// not session restore. The session override only matters for --context
// and --namespace.
func (o StartupOptions) HasCLIOverrides() bool {
	return o.Context != "" || len(o.Namespaces) > 0
}
