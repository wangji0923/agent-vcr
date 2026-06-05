package cli

import (
	"fmt"

	"github.com/agent-vcr/agent-vcr/internal/doctor"
	"github.com/spf13/cobra"
)

func newDoctorCommand(rootOpts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local agent-vcr environment diagnostics.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := doctor.Run(doctor.Options{
				ProjectDir: rootOpts.ProjectDir,
				ConfigPath: rootOpts.Config,
			})
			if err != nil {
				return err
			}
			if rootOpts.JSON {
				data, err := doctor.RenderJSON(result)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), doctor.RenderHuman(result))
			return err
		},
	}
}
