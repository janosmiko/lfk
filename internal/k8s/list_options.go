package k8s

// ListOption configures how a single GetResources call is routed. Options
// are additive flags on top of the client's InformerCacheMode -- they never
// force a call through the cache when the mode is off.
type ListOption func(*listOpts)

// listOpts is the resolved set of per-call options GetResources consults.
type listOpts struct {
	preferCache bool
}

// PreferCache marks a (context, GVR) "hot" for watch-tick refreshes.
// Hover previews must not pass this, or every resource type the cursor
// passes over would open a watch.
func PreferCache() ListOption {
	return func(o *listOpts) { o.preferCache = true }
}

// resolveListOpts applies every option in order and returns the result.
func resolveListOpts(opts []ListOption) listOpts {
	var o listOpts
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
