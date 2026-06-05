package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/config"
	"github.com/agent-vcr/agent-vcr/internal/redact"
	"github.com/agent-vcr/agent-vcr/internal/report"
	"github.com/agent-vcr/agent-vcr/internal/trace"
	"github.com/spf13/cobra"
)

type exportOptions struct {
	html     bool
	redacted bool
	output   string
}

func newExportCommand(rootOpts *Options) *cobra.Command {
	opts := &exportOptions{}
	cmd := &cobra.Command{
		Use:   "export <run-id|latest>",
		Short: "Export a recorded run.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !opts.html {
				return fmt.Errorf("only --html export is implemented")
			}
			projectDir, err := projectDir(rootOpts.ProjectDir)
			if err != nil {
				return err
			}
			cfg, _, err := config.Load(projectDir, rootOpts.Config)
			if err != nil {
				return err
			}
			runID, err := trace.ResolveRunID(projectDir, args[0])
			if err != nil {
				return err
			}
			reportRunID := runID
			if opts.redacted {
				redactedRunID := runID + "-redacted"
				outputDir := filepath.Join(projectDir, ".agent-vcr", "runs", redactedRunID)
				if err := redact.RedactRun(projectDir, runID, outputDir); err != nil {
					return err
				}
				reportRunID = redactedRunID
			}
			store, err := trace.OpenRun(projectDir, reportRunID)
			if err != nil {
				return err
			}
			outputPath := opts.output
			if strings.TrimSpace(outputPath) == "" {
				outputPath = filepath.Join(store.RunDir, "report.html")
			} else if !filepath.IsAbs(outputPath) {
				outputPath = filepath.Join(projectDir, outputPath)
			}
			if info, err := os.Stat(outputPath); err == nil && info.IsDir() {
				outputPath = filepath.Join(outputPath, "report.html")
			}
			data, err := report.Load(projectDir, reportRunID, cfg)
			if err != nil {
				return err
			}
			if err := report.WriteHTMLFile(data, outputPath); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), outputPath)
			return err
		},
	}
	cmd.Flags().BoolVar(&opts.html, "html", false, "export an offline HTML report")
	cmd.Flags().BoolVar(&opts.redacted, "redacted", false, "create a redacted run copy before exporting")
	cmd.Flags().StringVar(&opts.output, "output", "", "report output path")
	return cmd
}
