package cli

import (
	"encoding/json"
	"fmt"

	"github.com/agent-vcr/agent-vcr/internal/analysis"
	"github.com/agent-vcr/agent-vcr/internal/config"
	"github.com/spf13/cobra"
)

type checkOptions struct {
	ci bool
}

type checkExitError struct {
	code int
}

func (e checkExitError) Error() string {
	return fmt.Sprintf("agent-vcr check failed with exit code %d", e.code)
}

func (e checkExitError) ExitCode() int {
	return e.code
}

func newCheckCommand(rootOpts *Options) *cobra.Command {
	opts := &checkOptions{}
	cmd := &cobra.Command{
		Use:   "check <run-id|latest>",
		Short: "Check a recorded run against policy rules.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := config.Load(rootOpts.ProjectDir, rootOpts.Config)
			if err != nil {
				return err
			}
			run, err := analysis.LoadRunData(rootOpts.ProjectDir, args[0])
			if err != nil {
				return err
			}
			result := analysis.CheckRun(run, cfg)
			if rootOpts.JSON {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(result); err != nil {
					return err
				}
			} else {
				if _, err := cmd.OutOrStdout().Write([]byte(analysis.RenderCheckText(result, opts.ci))); err != nil {
					return err
				}
			}
			if opts.ci && !result.Passed {
				return checkExitError{code: 1}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.ci, "ci", false, "exit non-zero when check fails")
	return cmd
}
