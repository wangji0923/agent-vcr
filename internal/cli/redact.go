package cli

import (
	"fmt"
	"path/filepath"

	"github.com/agent-vcr/agent-vcr/internal/redact"
	"github.com/agent-vcr/agent-vcr/internal/trace"
	"github.com/spf13/cobra"
)

type redactOptions struct {
	output string
}

func newRedactCommand(rootOpts *Options) *cobra.Command {
	opts := &redactOptions{}
	cmd := &cobra.Command{
		Use:   "redact <run-id|latest>",
		Short: "Create a redacted copy of a recorded run.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectDir, err := projectDir(rootOpts.ProjectDir)
			if err != nil {
				return err
			}
			runID, err := trace.ResolveRunID(projectDir, args[0])
			if err != nil {
				return err
			}
			outputDir := opts.output
			if outputDir == "" {
				outputDir = filepath.Join(projectDir, ".agent-vcr", "runs", runID+"-redacted")
			} else if !filepath.IsAbs(outputDir) {
				outputDir = filepath.Join(projectDir, outputDir)
			}
			if err := redact.RedactRun(projectDir, runID, outputDir); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), outputDir)
			return err
		},
	}
	cmd.Flags().StringVar(&opts.output, "output", "", "redacted run output directory")
	return cmd
}
