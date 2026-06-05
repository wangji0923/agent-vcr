package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	runlist "github.com/agent-vcr/agent-vcr/internal/list"
	"github.com/spf13/cobra"
)

func newListCommand(rootOpts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List recorded runs.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			runs, err := runlist.Runs(rootOpts.ProjectDir)
			if err != nil {
				return err
			}
			if rootOpts.JSON {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(runs)
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderRunList(runs))
			return err
		},
	}
}

func renderRunList(runs []runlist.Summary) string {
	if len(runs) == 0 {
		return "No runs found.\n"
	}
	headers := []string{"ID", "Source", "Status", "Started", "Ended", "Tools", "Delta", "Tests"}
	rows := make([][]string, 0, len(runs)+1)
	rows = append(rows, headers)
	for _, run := range runs {
		rows = append(rows, []string{
			run.RunID,
			run.Source,
			run.Status,
			formatTimePtr(run.StartedAt),
			formatTimePtr(run.EndedAt),
			fmt.Sprintf("%d", run.ToolCalls),
			fmt.Sprintf("%d", run.ChangedFiles),
			run.TestCommandSummary,
		})
	}
	widths := make([]int, len(headers))
	for _, row := range rows {
		for i, value := range row {
			if len(value) > widths[i] {
				widths[i] = len(value)
			}
		}
	}

	var out strings.Builder
	for _, row := range rows {
		for i, value := range row {
			if i > 0 {
				out.WriteString("  ")
			}
			if i == len(row)-1 {
				out.WriteString(value)
				continue
			}
			out.WriteString(value)
			out.WriteString(strings.Repeat(" ", widths[i]-len(value)))
		}
		out.WriteByte('\n')
	}
	return out.String()
}

func formatTimePtr(value *time.Time) string {
	if value == nil || value.IsZero() {
		return "-"
	}
	return value.UTC().Format(time.RFC3339)
}
