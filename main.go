package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof" // registers /debug/pprof/* under DefaultServeMux when LFK_PPROF_ADDR is set
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/janosmiko/lfk/internal/app"
	"github.com/janosmiko/lfk/internal/completion"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/janosmiko/lfk/internal/version"
)

func main() {
	var cliOpts app.StartupOptions

	rootCmd := &cobra.Command{
		Use:   "lfk",
		Short: "Lightning Fast Kubernetes navigator",
		Long: `lfk is a keyboard-focused terminal user interface for navigating and managing Kubernetes clusters.

File locations:
  Config: ~/.config/lfk/config.yaml  (or $XDG_CONFIG_HOME/lfk/config.yaml)
  State:  ~/.local/state/lfk/        (bookmarks, session, history)
  Logs:   ~/.local/share/lfk/lfk.log
  Override dirs for portable installs: LFK_CONFIG_DIR, LFK_STATE_DIR, LFK_DATA_DIR`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI(cliOpts)
		},
		// Silence cobra's own usage/error printing so the TUI is not disrupted.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.Flags().StringVar(&cliOpts.Context, "context", "", "Kubernetes context to use")
	rootCmd.Flags().StringArrayVar(&cliOpts.UnionContexts, "union-context", nil, "Cluster context to include in union view (repeatable; requires --namespace)")
	rootCmd.Flags().StringVar(&cliOpts.UnionSet, "union-set", "", "Named union_sets entry from config to expand into a union view (mutex with --union-context and --context; --namespace overrides the set's namespace)")
	rootCmd.Flags().StringSliceVarP(&cliOpts.Namespaces, "namespace", "n", nil, "Namespace(s) to filter (repeatable, disables all-namespaces mode)")
	rootCmd.Flags().StringVar(&cliOpts.Kubeconfig, "kubeconfig", "", "Path to kubeconfig file (overrides default discovery)")
	rootCmd.Flags().StringArrayVar(&cliOpts.KubeconfigDirs, "kubeconfig-dir", nil, "Directory to scan for kubeconfig files instead of ~/.kube/config.d/. Repeatable: pass multiple flags to merge several directories. Also accepts KUBECONFIG_DIR env var (colon-separated). ~/ is expanded against $HOME.")
	rootCmd.Flags().StringVarP(&cliOpts.Config, "config", "c", "", "Path to config file (overrides default ~/.config/lfk/config.yaml)")
	rootCmd.Flags().BoolVar(&cliOpts.NoMouse, "no-mouse", false, "Disable mouse capture (enables native terminal text selection)")
	rootCmd.Flags().BoolVar(&cliOpts.NoColor, "no-color", false, "Disable foreground/background colors; keep bold/reverse for visibility. Also honors the NO_COLOR env var.")
	rootCmd.Flags().BoolVar(&cliOpts.ReadOnly, "read-only", false, "Disable all mutating actions (delete/edit/scale/restart/exec/port-forward/drain/cordon). Also configurable as read_only: true (global) or clusters.<ctx>.read_only (per-context) in config.")
	rootCmd.Flags().DurationVar(&cliOpts.WatchInterval, "watch-interval", 0, "Watch mode polling interval (e.g. 500ms, 2s, 1m). Clamped to [500ms, 10m]. Overrides config.")

	completion.RegisterShellCompletions(rootCmd)

	rootCmd.Version = version.Full()
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version.Full())
		},
	}
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(completion.NewCompletionCommand(rootCmd))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runTUI initializes the Kubernetes client, logger, and starts the Bubbletea TUI.
func runTUI(opts app.StartupOptions) error {
	// Silence klog (Kubernetes client library) to prevent it from writing
	// error messages to stderr which corrupts the TUI output.
	// Initially discard; after logger init, redirect to our log file.
	klog.InitFlags(nil)
	_ = flag.Set("logtostderr", "false")
	_ = flag.Set("stderrthreshold", "FATAL")
	klog.SetOutput(io.Discard)
	defer klog.Flush()

	if opts.Config != "" {
		if _, err := os.Stat(opts.Config); err != nil {
			return fmt.Errorf("config file %q: %w", opts.Config, err)
		}
	}
	ui.LoadConfig(opts.Config)

	if opts.Kubeconfig != "" {
		if _, err := os.Stat(opts.Kubeconfig); err != nil {
			return fmt.Errorf("kubeconfig file %q: %w", opts.Kubeconfig, err)
		}
	}

	kubeconfigDirs := k8s.ResolveKubeconfigDirs(
		opts.KubeconfigDirs,
		os.Getenv("KUBECONFIG_DIR"),
		ui.ConfigKubeconfigDirs,
	)
	if err := k8s.ValidateKubeconfigDirs(kubeconfigDirs); err != nil {
		return err
	}

	client, err := k8s.NewClient(opts.Kubeconfig, kubeconfigDirs)
	if err != nil {
		return fmt.Errorf("initializing Kubernetes client: %w", err)
	}

	if opts.Context != "" && !client.ContextExists(opts.Context) {
		return fmt.Errorf("context %q not found in kubeconfig", opts.Context)
	}

	// CLI --no-color flag can force monochrome even if config and env don't.
	// (LoadConfig already honors the NO_COLOR env var and config field.)
	if opts.NoColor {
		ui.SetNoColor(true)
	}
	model.PinnedGroups = ui.ConfigPinnedGroups
	client.SetSecretLazyLoading(ui.ConfigSecretLazyLoading)
	client.SetKubesharkNamespace(ui.ConfigKubesharkNamespace)
	client.SetInformerCacheMode(k8s.InformerCacheMode(ui.ConfigInformerCacheMode))
	defer client.Shutdown()

	// --union-set expansion runs AFTER LoadConfig (it reads ui.ConfigUnionSets)
	// and BEFORE ValidateUnionOptions (which then enforces context existence
	// and the cluster cap on the resolved list).
	opts, err = app.ResolveUnionSet(opts, unionSetLookup(ui.ConfigUnionSets, client))
	if err != nil {
		return err
	}
	if err := app.ValidateUnionOptions(opts, client.ContextExists); err != nil {
		return err
	}

	if err := logger.Init(ui.ConfigLogPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not initialize logger: %v\n", err)
	}
	defer logger.Close()

	// Now that the logger is initialized, redirect klog output to our application log.
	klog.SetOutput(logger.KlogWriter())
	logger.Info("Application started",
		"informer_cache", ui.ConfigInformerCacheMode,
		"secret_lazy_loading", ui.ConfigSecretLazyLoading,
		"config_path", opts.Config,
	)

	// Optional pprof endpoint for debugging hot CPU paths (issue #206).
	// Off by default; set LFK_PPROF_ADDR=127.0.0.1:6060 to enable. The
	// address MUST resolve to a loopback host — anything else exposes
	// process internals (heap, goroutines, credentials in symbols) on
	// the network and we refuse to start.
	// Capture an idle profile with:
	//   go tool pprof -seconds=30 http://127.0.0.1:6060/debug/pprof/profile
	if addr := os.Getenv("LFK_PPROF_ADDR"); addr != "" {
		if err := validatePprofAddr(addr); err != nil {
			return fmt.Errorf("LFK_PPROF_ADDR: %w", err)
		}
		go func() {
			logger.Info("starting pprof", "addr", addr)
			srv := &http.Server{Addr: addr} //nolint:gosec // validated loopback-only above
			if err := srv.ListenAndServe(); err != nil {
				logger.Warn("pprof server stopped", "error", err)
			}
		}()
	}

	// Redirect os.Stderr to capture output from exec credential plugins (e.g., AWS SSO
	// errors from `aws eks get-token`). Without this, subprocess stderr output goes
	// directly to the terminal and either corrupts the TUI or is lost.
	stderrCapture := logger.NewStderrCapture()
	origStderr := os.Stderr
	os.Stderr = stderrCapture.Writer()
	defer func() {
		os.Stderr = origStderr
		stderrCapture.Close()
	}()

	m := app.NewModel(client, opts)
	m.SetVersion(version.Short())
	m.SetStderrChan(stderrCapture.MsgChan)
	progOpts := []tea.ProgramOption{tea.WithAltScreen()}
	if !opts.NoMouse && ui.ConfigMouse {
		progOpts = append(progOpts, tea.WithMouseCellMotion())
	}
	if ui.ColorModeEnabled() {
		defer ui.DisableColorModeNotifications()
	}
	p := tea.NewProgram(m, progOpts...)

	if _, err := p.Run(); err != nil {
		os.Stderr = origStderr
		return fmt.Errorf("running application: %w", err)
	}

	return nil
}

// validatePprofAddr ensures LFK_PPROF_ADDR points at a loopback host so
// the debug pprof endpoint never gets exposed on the network. Accepts
// `localhost`, `127.0.0.1`, `::1`, and any other IP whose IsLoopback
// reports true. Rejects bind-all forms like `:6060`, `0.0.0.0:...`,
// and `[::]:...` because pprof leaks process internals (heap profile,
// goroutine stacks, env strings in binaries) and must not be reachable
// off-box.
func validatePprofAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid host:port %q: %w", addr, err)
	}
	if host == "" {
		return fmt.Errorf("must specify an explicit loopback host (got %q — bind-all form is refused)", addr)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("host %q is not an IP or 'localhost'", host)
	}
	if !ip.IsLoopback() {
		return fmt.Errorf("host %q is not a loopback address", host)
	}
	return nil
}

// unionSetLookup adapts the ui-package config slice to the
// app.UnionSetLookup signature ResolveUnionSet expects. Captured into a
// closure so the resolver doesn't grow an import on the ui package and
// stays unit-testable with a hand-rolled lookup. Flattens the per-cluster
// objects into the parallel (contexts, colors-map) shape the resolver
// passes downstream. The namespace resolver can also see explicit kubeconfig
// context namespaces through the already-constructed client.
func unionSetLookup(sets []ui.UnionSetConfig, client *k8s.Client) app.UnionSetLookup {
	return func(name string) (contexts []string, namespace string, colors map[string]string, ok bool) {
		var namespaceLookup func(string) (string, bool)
		if client != nil {
			namespaceLookup = client.ContextNamespace
		}
		for _, s := range sets {
			if s.Name != name {
				continue
			}
			ctxs, ns, cols, err := app.ExpandUnionSetConfig(s, namespaceLookup)
			if err != nil {
				// The lookup signature can't carry an error back to
				// ResolveUnionSet, so surface the malformed-set message via
				// the kubeconfig context name so it reads naturally in the
				// downstream "context not found" error path. Tests cover the
				// happy path; production users with this bug will see the
				// validation error at startup with the offending set name.
				logger.Error("union set is malformed", "error", err)
				return nil, "", nil, false
			}
			return ctxs, ns, cols, true
		}
		return nil, "", nil, false
	}
}
