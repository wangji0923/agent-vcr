package cli

import (
	"context"
	"fmt"

	_ "github.com/agent-vcr/agent-vcr/internal/adapters/codex"
	_ "github.com/agent-vcr/agent-vcr/internal/adapters/generic"
	"github.com/agent-vcr/agent-vcr/internal/process"
	"github.com/agent-vcr/agent-vcr/internal/record"
	"github.com/spf13/cobra"
)

type recordOptions struct {
	name          string
	adapter       string
	cwd           string
	captureStdout bool
	captureStderr bool
}

func newRecordCommand(rootOpts *Options) *cobra.Command {
	opts := &recordOptions{
		adapter:       record.AdapterAuto,
		captureStdout: true,
		captureStderr: true,
	}

	cmd := &cobra.Command{
		Use:   "record [flags] -- <command> [args...]",
		Short: "Record an agent run.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := record.Run(context.Background(), record.Options{
				ProjectDir:    rootOpts.ProjectDir,
				ConfigPath:    rootOpts.Config,
				Name:          opts.name,
				Adapter:       opts.adapter,
				Cwd:           opts.cwd,
				CaptureStdout: opts.captureStdout,
				CaptureStderr: opts.captureStderr,
				Command:       args,
			})
			if err != nil {
				return err
			}
			for _, warning := range result.Warnings {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), warning)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "recorded run %s (%s)\n", result.RunID, result.Adapter)
			if result.ExitCode != 0 {
				return process.ExitCodeError{Code: result.ExitCode}
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.name, "name", "", "custom run name suffix")
	flags.StringVar(&opts.adapter, "adapter", record.AdapterAuto, "adapter: auto, codex-jsonl, or generic-cli")
	flags.StringVar(&opts.cwd, "cwd", "", "child process working directory")
	flags.BoolVar(&opts.captureStdout, "capture-stdout", true, "capture child stdout to a blob")
	flags.BoolVar(&opts.captureStderr, "capture-stderr", true, "capture child stderr to a blob")

	return cmd
}
