package cli

import (
	"io"

	"github.com/agent-vcr/agent-vcr/internal/adapters/codex"
	"github.com/spf13/cobra"
)

func newHookCommand(opts *Options) *cobra.Command {
	var adapterName string
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Handle agent lifecycle hook events.",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch adapterName {
			case codex.AdapterName:
				_ = codex.RunHook(cmd.Context(), codex.HookRunOptions{
					Stdin:      cmd.InOrStdin(),
					ProjectDir: opts.ProjectDir,
					ConfigPath: opts.Config,
				})
			default:
				_, _ = io.Copy(io.Discard, cmd.InOrStdin())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&adapterName, "adapter", "", "adapter name")
	return cmd
}
