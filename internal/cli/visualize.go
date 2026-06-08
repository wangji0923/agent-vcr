package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/visualize"
	"github.com/spf13/cobra"
)

type visualizeCommandOptions struct {
	html    bool
	output  string
	noCache bool
	maxRuns int
}

func newVisualizeCommand(rootOpts *Options) *cobra.Command {
	opts := &visualizeCommandOptions{maxRuns: visualize.MaxRecommendedRuns}
	cmd := &cobra.Command{
		Use:   "visualize <run-id|latest> [run-id...]",
		Short: "Build a behavior visualization report.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.html && rootOpts.JSON {
				return fmt.Errorf("--html and --json cannot be used together")
			}
			if opts.maxRuns <= 0 {
				return fmt.Errorf("--max-runs must be greater than 0")
			}
			project, err := projectDir(rootOpts.ProjectDir)
			if err != nil {
				return err
			}
			report, err := visualize.BuildReport(cmd.Context(), visualize.LoadOptions{
				ProjectDir: project,
				RunIDs:     args,
				NoCache:    opts.noCache,
				MaxRuns:    opts.maxRuns,
			})
			if err != nil {
				return err
			}
			switch {
			case opts.html:
				outputPath := resolveVisualizeOutputPath(project, report, opts.output, ".html")
				if err := visualize.WriteHTMLFile(report, outputPath); err != nil {
					return err
				}
				_, err := fmt.Fprintln(cmd.OutOrStdout(), outputPath)
				return err
			case rootOpts.JSON:
				if strings.TrimSpace(opts.output) != "" {
					outputPath := resolveVisualizeOutputPath(project, report, opts.output, ".json")
					return visualize.WriteJSONFile(report, outputPath)
				}
				return visualize.WriteJSON(cmd.OutOrStdout(), report)
			default:
				if strings.TrimSpace(opts.output) != "" {
					outputPath := resolveVisualizeOutputPath(project, report, opts.output, ".txt")
					if err := visualize.WriteSummaryFile(report, outputPath); err != nil {
						return err
					}
					_, err := fmt.Fprintln(cmd.OutOrStdout(), outputPath)
					return err
				}
				_, err := fmt.Fprint(cmd.OutOrStdout(), visualize.RenderSummary(report))
				return err
			}
		},
	}
	cmd.Flags().BoolVar(&opts.html, "html", false, "write an offline HTML visualization")
	cmd.Flags().StringVar(&opts.output, "output", "", "output path")
	cmd.Flags().BoolVar(&opts.noCache, "no-cache", false, "rebuild behavior from trace without reading or writing cache")
	cmd.Flags().IntVar(&opts.maxRuns, "max-runs", visualize.MaxRecommendedRuns, "maximum number of runs to visualize")
	return cmd
}

func resolveVisualizeOutputPath(projectDir string, report visualize.VisualReport, outputPath string, ext string) string {
	if strings.TrimSpace(outputPath) == "" {
		if ext == ".html" {
			return visualize.DefaultHTMLOutputPath(projectDir, report)
		}
		return ""
	}
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(projectDir, outputPath)
	}
	if info, err := os.Stat(outputPath); err == nil && info.IsDir() {
		name := "visual-report" + ext
		if ext == ".txt" {
			name = "visual-summary.txt"
		}
		outputPath = filepath.Join(outputPath, name)
	}
	return outputPath
}
