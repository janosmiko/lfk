package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/janosmiko/lfk/internal/app"
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
  Logs:   ~/.local/share/lfk/lfk.log`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI(cliOpts)
		},
		// Silence cobra's own usage/error printing so the TUI is not disrupted.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.Flags().StringVar(&cliOpts.Context, "context", "", "Kubernetes context to use")
	rootCmd.Flags().StringSliceVarP(&cliOpts.Namespaces, "namespace", "n", nil, "Namespace(s) to filter (repeatable, disables all-namespaces mode)")
	rootCmd.Flags().StringVar(&cliOpts.Kubeconfig, "kubeconfig", "", "Path to kubeconfig file (overrides default discovery)")
	rootCmd.Flags().StringVarP(&cliOpts.Config, "config", "c", "", "Path to config file (overrides default ~/.config/lfk/config.yaml)")
	rootCmd.Flags().BoolVar(&cliOpts.NoMouse, "no-mouse", false, "Disable mouse capture (enables native terminal text selection)")
	rootCmd.Flags().BoolVar(&cliOpts.NoColor, "no-color", false, "Disable foreground/background colors; keep bold/reverse for visibility. Also honors the NO_COLOR env var.")
	rootCmd.Flags().BoolVar(&cliOpts.ReadOnly, "read-only", false, "Disable all mutating actions (delete/edit/scale/restart/exec/port-forward/drain/cordon). Also configurable as read_only: true (global) or clusters.<ctx>.read_only (per-context) in config.")
	rootCmd.Flags().DurationVar(&cliOpts.WatchInterval, "watch-interval", 0, "Watch mode polling interval (e.g. 500ms, 2s, 1m). Clamped to [500ms, 10m]. Overrides config.")

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

	if opts.Kubeconfig != "" {
		if _, err := os.Stat(opts.Kubeconfig); err != nil {
			return fmt.Errorf("kubeconfig file %q: %w", opts.Kubeconfig, err)
		}
	}
	if opts.Config != "" {
		if _, err := os.Stat(opts.Config); err != nil {
			return fmt.Errorf("config file %q: %w", opts.Config, err)
		}
	}

	client, err := k8s.NewClient(opts.Kubeconfig)
	if err != nil {
		return fmt.Errorf("initializing Kubernetes client: %w", err)
	}

	if opts.Context != "" && !client.ContextExists(opts.Context) {
		return fmt.Errorf("context %q not found in kubeconfig", opts.Context)
	}

	ui.LoadConfig(opts.Config)
	// CLI --no-color flag can force monochrome even if config and env don't.
	// (LoadConfig already honors the NO_COLOR env var and config field.)
	if opts.NoColor {
		ui.SetNoColor(true)
	}
	model.PinnedGroups = ui.ConfigPinnedGroups
	client.SetSecretLazyLoading(ui.ConfigSecretLazyLoading)

	if err := logger.Init(ui.ConfigLogPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not initialize logger: %v\n", err)
	}
	defer logger.Close()

	// Now that the logger is initialized, redirect klog output to our application log.
	klog.SetOutput(logger.KlogWriter())
	logger.Info("Application started")

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
