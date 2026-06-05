package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/adapters"
	"github.com/agent-vcr/agent-vcr/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newInitCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize agent-vcr configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	var scope string
	var force bool
	codexCmd := &cobra.Command{
		Use:   "codex",
		Short: "Install Codex hook recording.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectDir, err := projectDir(opts.ProjectDir)
			if err != nil {
				return err
			}
			adapter, ok := adapters.Get("codex")
			if !ok {
				return fmt.Errorf("codex adapter is not registered")
			}
			_, _ = adapter.Probe(context.Background())
			if err := adapter.Install(context.Background(), adapters.InstallOptions{
				Scope:      scope,
				ProjectDir: projectDir,
				Force:      force,
			}); err != nil {
				return err
			}
			if err := ensureProjectConfig(projectDir); err != nil {
				return err
			}
			if err := ensureGitignore(projectDir); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), `Installed Codex hooks.
Next:
  1. Run `+"`codex`"+` in this repo.
  2. Open `+"`/hooks`"+` and trust the agent-vcr hook.
  3. Use Codex normally.`)
			return err
		},
	}
	codexCmd.Flags().StringVar(&scope, "scope", "project", "hook install scope: project or user")
	codexCmd.Flags().BoolVar(&force, "force", false, "update existing agent-vcr hook settings")
	cmd.AddCommand(codexCmd)
	return cmd
}

func projectDir(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		value = cwd
	}
	return filepath.Abs(value)
}

func ensureProjectConfig(projectDir string) error {
	path := filepath.Join(projectDir, ".agent-vcr", "config.yml")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(config.Default())
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func ensureGitignore(projectDir string) error {
	path := filepath.Join(projectDir, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := string(data)
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == ".agent-vcr/" {
			return nil
		}
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += ".agent-vcr/\n"
	return os.WriteFile(path, []byte(text), 0o644)
}
