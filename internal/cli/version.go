package cli

import (
	"encoding/json"
	"fmt"

	"github.com/agent-vcr/agent-vcr/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.JSON {
				encoded, err := json.Marshal(version.Get())
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
				return err
			}

			_, err := fmt.Fprintln(cmd.OutOrStdout(), version.String())
			return err
		},
	}
}
