package cli

import (
	"encoding/json"

	"github.com/agent-vcr/agent-vcr/internal/analysis"
	"github.com/spf13/cobra"
)

func newDiffCommand(rootOpts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "diff run-a run-b",
		Short: "Diff two recorded runs.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			runA, err := analysis.LoadRunData(rootOpts.ProjectDir, args[0])
			if err != nil {
				return err
			}
			runB, err := analysis.LoadRunData(rootOpts.ProjectDir, args[1])
			if err != nil {
				return err
			}
			result := analysis.DiffRuns(runA, runB)
			if rootOpts.JSON {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(result)
			}
			_, err = cmd.OutOrStdout().Write([]byte(analysis.RenderDiffText(result)))
			return err
		},
	}
}
