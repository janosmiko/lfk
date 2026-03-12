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
	rootCmd := &cobra.Command{
		Use:   "lfk",
		Short: "Lightning Fast Kubernetes navigator",
		Long: `lfk is a keyboard-focused terminal user interface for navigating and managing Kubernetes clusters.

File locations:
  Config: ~/.config/lfk/config.yaml  (or $XDG_CONFIG_HOME/lfk/config.yaml)
  State:  ~/.local/state/lfk/        (bookmarks, session, history)
  Logs:   ~/.local/share/lfk/lfk.log`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI()
		},
		// Silence cobra's own usage/error printing so the TUI is not disrupted.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

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
func runTUI() error {
	// Silence klog (Kubernetes client library) to prevent it from writing
	// error messages to stderr which corrupts the TUI output.
	// Initially discard; after logger init, redirect to our log file.
	klog.InitFlags(nil)
	_ = flag.Set("logtostderr", "false")
	_ = flag.Set("stderrthreshold", "FATAL")
	klog.SetOutput(io.Discard)
	defer klog.Flush()

	client, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("initializing Kubernetes client: %w", err)
	}

	ui.LoadAndApplyTheme()
	model.PinnedGroups = ui.ConfigPinnedGroups

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

	model := app.NewModel(client)
	model.SetVersion(version.Short())
	model.SetStderrChan(stderrCapture.MsgChan)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		os.Stderr = origStderr
		return fmt.Errorf("running application: %w", err)
	}

	return nil
}
