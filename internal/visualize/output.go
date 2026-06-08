package visualize

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/behavior"
)

func BuildReport(ctx context.Context, opts LoadOptions) (VisualReport, error) {
	runs, err := LoadRuns(ctx, opts)
	if err != nil {
		return VisualReport{}, err
	}
	report, err := BuildReportSkeleton(runs, opts)
	if err != nil {
		return VisualReport{}, err
	}
	report.Alignment = AlignLanes(report.Lanes, AlignOptions{
		BaselineRunID:       report.Options.BaselineRunID,
		Divergences:         report.Divergences,
		MarkFirstDivergence: true,
	})
	report.FileAccess = BuildFileAccessCompare(report.Lanes)
	report.SearchScopes = BuildSearchScopeCompare(report.Lanes)
	report.Metrics = BuildMetricsCardGroups(report.Runs, metricsByRun(runs))
	report.PathGraph = BuildPathGraph(report.Lanes)
	report.Summary = summarizeVisualReport(report, firstDivergenceMarker(report.Divergences))
	if err := ValidateReport(&report); err != nil {
		return VisualReport{}, err
	}
	return report, nil
}

func WriteJSON(writer io.Writer, report VisualReport) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func WriteJSONFile(report VisualReport, outputPath string) error {
	return writeOutputFile(outputPath, func(writer io.Writer) error {
		return WriteJSON(writer, report)
	})
}

func RenderSummary(report VisualReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Behavior visualization: %s\n", report.Mode)
	fmt.Fprintf(&b, "Runs: %d\n", len(report.Runs))
	for _, run := range report.Runs {
		label := run.Label
		if strings.TrimSpace(label) == "" {
			label = run.RunID
		}
		fmt.Fprintf(&b, "- %s (%s): %d steps, status=%s\n", label, run.RunID, run.StepCount, run.Status)
	}
	fmt.Fprintf(&b, "Steps: %d total, %d significant\n", report.Summary.StepCount, report.Summary.SignificantStepCount)
	if report.Summary.FirstDivergence != nil {
		divergence := report.Summary.FirstDivergence
		fmt.Fprintf(&b, "First divergence: step %d, %s -> %s", divergence.StepIndex, divergence.BaselineRunID, divergence.CompareRunID)
		if strings.TrimSpace(divergence.Kind) != "" {
			fmt.Fprintf(&b, ", kind=%s", divergence.Kind)
		}
		b.WriteByte('\n')
		if strings.TrimSpace(divergence.Summary) != "" {
			fmt.Fprintf(&b, "Divergence summary: %s\n", singleLine(divergence.Summary, 200))
		}
	} else if report.Mode == RenderModeCompare {
		b.WriteString("First divergence: none detected\n")
	}
	fmt.Fprintf(&b, "Alignment rows: %d\n", len(report.Alignment))
	fmt.Fprintf(&b, "File access rows: %d\n", len(report.FileAccess.Rows))
	fmt.Fprintf(&b, "Metrics cards: %d\n", report.Summary.MetricsCardCount)
	if len(report.Warnings) > 0 {
		b.WriteString("Warnings:\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&b, "- %s\n", warning)
		}
	}
	return b.String()
}

func WriteSummaryFile(report VisualReport, outputPath string) error {
	return writeOutputFile(outputPath, func(writer io.Writer) error {
		_, err := io.WriteString(writer, RenderSummary(report))
		return err
	})
}

func DefaultHTMLOutputPath(projectDir string, report VisualReport) string {
	runID := "visual-report"
	if len(report.Runs) > 0 && strings.TrimSpace(report.Runs[0].RunID) != "" {
		runID = report.Runs[0].RunID
	}
	generatedAt := report.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	name := fmt.Sprintf("visual-%s.html", generatedAt.UTC().Format("20060102T150405Z"))
	return filepath.Join(projectDir, ".agent-vcr", "runs", runID, "visual", name)
}

func WriteHTMLFile(report VisualReport, outputPath string) error {
	data, err := RenderHTML(&report, HTMLOptions{})
	if err != nil {
		return err
	}
	return writeOutputFile(outputPath, func(writer io.Writer) error {
		_, err := writer.Write(data)
		return err
	})
}

func metricsByRun(runs []LoadedRun) map[string]*behavior.Metrics {
	out := make(map[string]*behavior.Metrics, len(runs))
	for _, run := range runs {
		metrics := run.Metrics
		out[run.RunID] = &metrics
	}
	return out
}

func writeOutputFile(outputPath string, write func(io.Writer) error) error {
	if strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return write(file)
}

func singleLine(value string, maxLen int) string {
	value = strings.Join(strings.Fields(value), " ")
	if maxLen > 3 && len(value) > maxLen {
		return value[:maxLen-3] + "..."
	}
	return value
}
