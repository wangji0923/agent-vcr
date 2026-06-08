package cli

import (
	"github.com/agent-vcr/agent-vcr/internal/version"
	"github.com/spf13/cobra"
)

type Options struct {
	ProjectDir string
	Config     string
	JSON       bool
	Verbose    bool
}

func NewRootCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:           "agent-vcr",
		Short:         "Behavior diff for AI coding agents.",
		Long:          "Behavior diff for AI coding agents.",
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.SetVersionTemplate("{{.Version}}\n")
	cmd.CompletionOptions.DisableDefaultCmd = true

	flags := cmd.PersistentFlags()
	flags.StringVar(&opts.ProjectDir, "project-dir", "", "project directory")
	flags.StringVar(&opts.Config, "config", "", "config file")
	flags.BoolVar(&opts.JSON, "json", false, "write JSON output")
	flags.BoolVar(&opts.Verbose, "verbose", false, "write verbose diagnostics")

	cmd.AddCommand(
		newVersionCommand(opts),
		newDoctorCommand(opts),
		newInitCommand(opts),
		newHookCommand(opts),
		newRecordCommand(opts),
		newListCommand(opts),
		newReplayCommand(opts),
		newDiffCommand(opts),
		newBehaviorCommand(opts),
		newVisualizeCommand(opts),
		newCheckCommand(opts),
		newExportCommand(opts),
		newRedactCommand(opts),
	)

	return cmd
}
