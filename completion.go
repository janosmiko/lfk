package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/k8s"
)

func registerShellCompletions(rootCmd *cobra.Command) {
	_ = rootCmd.RegisterFlagCompletionFunc("union-context", completeUnionContextFlag)
	_ = rootCmd.RegisterFlagCompletionFunc("union-set", completeUnionSetFlag)
}

func newCompletionCommand(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script",
		Long: `Generate a shell completion script for lfk.

		For zsh:
  source <(lfk completion zsh)`,
		Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{
			"bash",
			"zsh",
			"fish",
			"powershell",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(out)
			case "zsh":
				return rootCmd.GenZshCompletion(out)
			case "fish":
				return rootCmd.GenFishCompletion(out, true)
			case "powershell":
				return rootCmd.GenPowerShellCompletion(out)
			default:
				return fmt.Errorf("unsupported shell %q", args[0])
			}
		},
	}
	return cmd
}

func completeUnionContextFlag(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
	kubeconfigDirs, _ := cmd.Flags().GetStringArray("kubeconfig-dir")
	completions, err := completeKubeContexts(kubeconfig, kubeconfigDirs, selectedUnionContexts(cmd), toComplete)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

func completeUnionSetFlag(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	configPath, _ := cmd.Flags().GetString("config")
	completions, err := completeUnionSets(configPath, toComplete)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

func selectedUnionContexts(cmd *cobra.Command) map[string]struct{} {
	selected := map[string]struct{}{}
	values, err := cmd.Flags().GetStringArray("union-context")
	if err != nil {
		return selected
	}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			selected[value] = struct{}{}
		}
	}
	return selected
}

func completeKubeContexts(kubeconfigOverride string, kubeconfigDirs []string, selected map[string]struct{}, prefix string) ([]string, error) {
	client, err := k8s.NewClient(kubeconfigOverride, kubeconfigDirs)
	if err != nil {
		return nil, err
	}
	items, err := client.GetContexts()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		name := item.Name
		if _, ok := selected[name]; ok {
			continue
		}
		if strings.HasPrefix(name, prefix) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

func completeUnionSets(configOverride, prefix string) ([]string, error) {
	sets, err := loadUnionSetNames(configOverride)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(sets))
	for _, name := range sets {
		if strings.HasPrefix(name, prefix) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

func loadUnionSetNames(configOverride string) ([]string, error) {
	configPath, ok := completionConfigPath(configOverride)
	if !ok {
		return nil, nil
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	names, err := decodeUnionSetNames(data)
	if err != nil {
		return nil, err
	}
	return names, nil
}

func decodeUnionSetNames(data []byte) ([]string, error) {
	var cfg struct {
		UnionSets []struct {
			Name string `json:"name" yaml:"name"`
		} `json:"union_sets" yaml:"union_sets"`
	}
	if err := yaml.Unmarshal(data, &cfg); err == nil && cfg.UnionSets != nil {
		seen := make(map[string]struct{}, len(cfg.UnionSets))
		names := make([]string, 0, len(cfg.UnionSets))
		for _, set := range cfg.UnionSets {
			name := strings.TrimSpace(set.Name)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
		return names, nil
	}

	var mapped struct {
		UnionSets map[string]any `json:"union_sets" yaml:"union_sets"`
	}
	if err := yaml.Unmarshal(data, &mapped); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(mapped.UnionSets))
	for name := range mapped.UnionSets {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func completionConfigPath(configOverride string) (string, bool) {
	if configOverride != "" {
		return configOverride, true
	}
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "lfk", "config.yaml"), true
}
