package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/agent-vcr/agent-vcr/internal/analysis"
	"github.com/agent-vcr/agent-vcr/internal/behavior"
	"github.com/agent-vcr/agent-vcr/internal/trace"
	"github.com/spf13/cobra"
)

type behaviorCommandOptions struct {
	noCache bool
}

type behaviorRunView struct {
	Ref           string                 `json:"ref,omitempty"`
	RunID         string                 `json:"run_id"`
	CacheHit      bool                   `json:"cache_hit"`
	Timeline      behavior.Timeline      `json:"timeline"`
	Signature     behavior.Signature     `json:"signature"`
	MetricsReport behavior.MetricsReport `json:"metrics_report"`
	Warnings      []string               `json:"warnings,omitempty"`
}

type behaviorDiffView struct {
	RunA            string                `json:"run_a"`
	RunB            string                `json:"run_b"`
	FirstDivergence *behavior.Divergence  `json:"first_divergence,omitempty"`
	Summary         string                `json:"summary"`
	MetricsDelta    behavior.MetricsDelta `json:"metrics_delta"`
	Warnings        []string              `json:"warnings,omitempty"`
}

func newBehaviorCommand(rootOpts *Options) *cobra.Command {
	opts := &behaviorCommandOptions{}
	cmd := &cobra.Command{
		Use:   "behavior <run-id|latest>",
		Short: "Show behavior timeline, signature, and metrics.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			view, err := loadBehaviorRun(cmd.Context(), rootOpts.ProjectDir, args[0], opts.noCache)
			if err != nil {
				return err
			}
			if rootOpts.JSON {
				return writeJSON(cmd, view.Signature)
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderBehaviorTimeline(view))
			return err
		},
	}
	cmd.PersistentFlags().BoolVar(&opts.noCache, "no-cache", false, "rebuild behavior from trace without reading or writing cache")

	cmd.AddCommand(newBehaviorDiffCommand(rootOpts, opts))
	cmd.AddCommand(newBehaviorMetricsCommand(rootOpts, opts))
	return cmd
}

func newBehaviorDiffCommand(rootOpts *Options, opts *behaviorCommandOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "diff run-a run-b",
		Short: "Diff two runs at the behavior level.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := loadBehaviorDiff(cmd.Context(), rootOpts.ProjectDir, args[0], args[1], opts.noCache)
			if err != nil {
				return err
			}
			if rootOpts.JSON {
				return writeJSON(cmd, result)
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderBehaviorDiff(result))
			return err
		},
	}
}

func newBehaviorMetricsCommand(rootOpts *Options, opts *behaviorCommandOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "metrics <run-id|latest>",
		Short: "Show behavior metrics for a run.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			view, err := loadBehaviorRun(cmd.Context(), rootOpts.ProjectDir, args[0], opts.noCache)
			if err != nil {
				return err
			}
			if rootOpts.JSON {
				return writeJSON(cmd, view.MetricsReport)
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), renderBehaviorMetrics(view.MetricsReport))
			return err
		},
	}
}

func loadBehaviorRun(ctx context.Context, projectDir string, ref string, noCache bool) (behaviorRunView, error) {
	runID, err := trace.ResolveRunID(projectDir, ref)
	if err != nil {
		return behaviorRunView{}, fmt.Errorf("resolve behavior run %q: %w", ref, err)
	}
	store, err := trace.OpenRun(projectDir, runID)
	if err != nil {
		return behaviorRunView{}, fmt.Errorf("open behavior run %q: %w", runID, err)
	}
	tracePath := store.Path("trace.ndjson")
	if _, err := os.Stat(tracePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return behaviorRunView{}, fmt.Errorf("trace not found for run %q: %s", runID, tracePath)
		}
		return behaviorRunView{}, fmt.Errorf("stat trace for run %q: %w", runID, err)
	}
	events, err := analysis.ReadTraceFile(tracePath)
	if err != nil {
		return behaviorRunView{}, fmt.Errorf("read trace for run %q: %w", runID, err)
	}

	commandClassifier := behavior.NewDefaultCommandClassifier()
	pathClassifier := behavior.NewDefaultPathClassifier()
	extractor := behavior.NewEventExtractor(commandClassifier, pathClassifier)
	extracted, err := extractor.Extract(ctx, behavior.ExtractInput{
		RunID:  runID,
		Events: events,
	})
	if err != nil {
		return behaviorRunView{}, fmt.Errorf("extract behavior for run %q: %w", runID, err)
	}
	timeline := extracted.Timeline
	metricsReport := behavior.ComputeMetricsWithOptions(timeline, behavior.MetricsOptions{PathClassifier: pathClassifier})
	signature, cacheHit, err := loadOrBuildBehaviorSignature(store.RunDir, timeline, metricsReport.Metrics, noCache)
	if err != nil {
		return behaviorRunView{}, fmt.Errorf("build behavior signature for run %q: %w", runID, err)
	}

	warnings := append([]string{}, extracted.Warnings...)
	warnings = append(warnings, timeline.Warnings...)
	return behaviorRunView{
		Ref:           ref,
		RunID:         runID,
		CacheHit:      cacheHit,
		Timeline:      timeline,
		Signature:     signature,
		MetricsReport: metricsReport,
		Warnings:      uniqueCLIStrings(warnings),
	}, nil
}

func loadOrBuildBehaviorSignature(runDir string, timeline behavior.Timeline, metrics behavior.Metrics, noCache bool) (behavior.Signature, bool, error) {
	opts := behavior.SignatureOptions{NormalizeUserPaths: true}
	traceHash, err := behavior.ComputeRunTraceHash(runDir)
	if err != nil {
		return behavior.Signature{}, false, err
	}
	if !noCache {
		cached, err := behavior.ReadSignatureCache(runDir)
		if err == nil && (traceHash == "" || cached.SourceTraceHash == traceHash) {
			cached.Metrics = metrics
			return cached, true, nil
		}
		if err != nil && !errors.Is(err, behavior.ErrSignatureCacheMiss) {
			// Rebuild below; corrupt or stale behavior cache must not block trace inspection.
		}
	}

	signature := behavior.BuildSignatureFromTimeline(timeline, opts)
	signature.SourceTraceHash = traceHash
	signature.Metrics = metrics
	if !noCache {
		if err := behavior.WriteSignatureCache(runDir, signature); err != nil {
			return behavior.Signature{}, false, err
		}
	}
	return signature, false, nil
}

func loadBehaviorDiff(ctx context.Context, projectDir string, refA string, refB string, noCache bool) (behaviorDiffView, error) {
	runA, err := loadBehaviorRun(ctx, projectDir, refA, noCache)
	if err != nil {
		return behaviorDiffView{}, err
	}
	runB, err := loadBehaviorRun(ctx, projectDir, refB, noCache)
	if err != nil {
		return behaviorDiffView{}, err
	}

	diff := behavior.DiffSignatures(runA.Signature, runB.Signature, behavior.DiffOptions{
		IgnoreRawBehavior:  true,
		IgnoreProcessNoise: true,
	})
	return behaviorDiffView{
		RunA:            diff.RunA,
		RunB:            diff.RunB,
		FirstDivergence: diff.FirstDivergence,
		Summary:         diff.Summary.Message,
		MetricsDelta:    diff.MetricsDelta,
		Warnings:        uniqueCLIStrings(append(runA.Warnings, runB.Warnings...)),
	}, nil
}

func renderBehaviorTimeline(view behaviorRunView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Behavior timeline: %s\n", view.Ref)
	fmt.Fprintf(&b, "Run: %s\n", view.RunID)
	if view.CacheHit {
		b.WriteString("Cache: hit\n")
	} else {
		b.WriteString("Cache: rebuilt\n")
	}
	if len(view.Signature.Steps) == 0 {
		b.WriteString("No behavior steps found.\n")
	} else {
		for i, step := range view.Signature.Steps {
			fmt.Fprintf(&b, "%d. %-18s %s", i+1, step.Kind, behaviorStepDetail(step))
			if step.Result != "" && step.Result != behavior.ResultUnknown {
				fmt.Fprintf(&b, " -> %s", step.Result)
			}
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString("Metrics summary:\n")
	writeBehaviorMetricsSummary(&b, view.MetricsReport.Metrics)
	if len(view.Warnings) > 0 {
		b.WriteString("\nWarnings:\n")
		for _, warning := range view.Warnings {
			fmt.Fprintf(&b, "- %s\n", warning)
		}
	}
	return b.String()
}

func renderBehaviorMetrics(report behavior.MetricsReport) string {
	var b strings.Builder
	writeBehaviorMetricsSummary(&b, report.Metrics)
	if len(report.Facts.EditedDirectories) > 0 ||
		report.Facts.RepeatedSearches > 0 ||
		report.Facts.RepeatedCommands > 0 ||
		report.Facts.ShellCommands > 0 ||
		report.Facts.SkipValidation ||
		report.Facts.ContinuedAfterFailure ||
		report.Facts.RepeatedFailure ||
		report.Facts.CrossUnrelatedDirectories {
		b.WriteString("\nFacts:\n")
		fmt.Fprintf(&b, "  shell commands: %d\n", report.Facts.ShellCommands)
		fmt.Fprintf(&b, "  repeated searches: %d\n", report.Facts.RepeatedSearches)
		fmt.Fprintf(&b, "  repeated commands: %d\n", report.Facts.RepeatedCommands)
		fmt.Fprintf(&b, "  skip validation: %s\n", yesNo(report.Facts.SkipValidation))
		fmt.Fprintf(&b, "  continued after failure: %s\n", yesNo(report.Facts.ContinuedAfterFailure))
		fmt.Fprintf(&b, "  repeated failure: %s\n", yesNo(report.Facts.RepeatedFailure))
		fmt.Fprintf(&b, "  cross unrelated directories: %s\n", yesNo(report.Facts.CrossUnrelatedDirectories))
		if len(report.Facts.EditedDirectories) > 0 {
			fmt.Fprintf(&b, "  edited directories: %s\n", strings.Join(report.Facts.EditedDirectories, ", "))
		}
	}
	return b.String()
}

func renderBehaviorDiff(result behaviorDiffView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Behavior diff: %s -> %s\n", result.RunA, result.RunB)
	if result.FirstDivergence == nil {
		b.WriteString("First behavior divergence: unavailable\n")
	} else {
		fmt.Fprintf(&b, "First behavior divergence at step %d\n", result.FirstDivergence.Index+1)
		fmt.Fprintf(&b, "Kind: %s\n", result.FirstDivergence.Kind)
		if result.FirstDivergence.RunAStep != nil {
			fmt.Fprintf(&b, "Run A: %s %s\n", result.FirstDivergence.RunAStep.Kind, behaviorStepDetail(*result.FirstDivergence.RunAStep))
		}
		if result.FirstDivergence.RunBStep != nil {
			fmt.Fprintf(&b, "Run B: %s %s\n", result.FirstDivergence.RunBStep.Kind, behaviorStepDetail(*result.FirstDivergence.RunBStep))
		}
		if result.FirstDivergence.Summary != "" {
			fmt.Fprintf(&b, "Summary: %s\n", result.FirstDivergence.Summary)
		}
	}
	if result.Summary != "" {
		fmt.Fprintf(&b, "Summary: %s\n", result.Summary)
	}
	if len(result.Warnings) > 0 {
		b.WriteString("Warnings:\n")
		for _, warning := range result.Warnings {
			fmt.Fprintf(&b, "- %s\n", warning)
		}
	}
	return b.String()
}

func writeBehaviorMetricsSummary(b *strings.Builder, metrics behavior.Metrics) {
	b.WriteString("Context discipline:\n")
	fmt.Fprintf(b, "  read tests before edit: %s\n", yesNo(metrics.ContextDiscipline.ReadTestsBeforeEdit))
	fmt.Fprintf(b, "  legacy path touched: %s\n", yesNo(metrics.ContextDiscipline.LegacyPathTouched))
	fmt.Fprintf(b, "  unique files read: %d\n", metrics.ContextDiscipline.UniqueFilesRead)
	fmt.Fprintf(b, "  repeated reads: %d\n", metrics.ContextDiscipline.RepeatedReads)
	b.WriteString("Validation:\n")
	fmt.Fprintf(b, "  ran tests: %s\n", yesNo(metrics.Validation.RanAnyTests))
	fmt.Fprintf(b, "  ran tests after edit: %s\n", yesNo(metrics.Validation.RanTestsAfterEdit))
	fmt.Fprintf(b, "  failed test runs: %d\n", metrics.Validation.FailedTestRuns)
	fmt.Fprintf(b, "  ignored failed command: %s\n", yesNo(metrics.Validation.IgnoredFailedCommand))
	b.WriteString("Edit scope:\n")
	fmt.Fprintf(b, "  files edited: %d\n", metrics.EditScope.FilesEdited)
	fmt.Fprintf(b, "  source files edited: %d\n", metrics.EditScope.SourceFilesEdited)
	fmt.Fprintf(b, "  test files edited: %d\n", metrics.EditScope.TestFilesEdited)
	fmt.Fprintf(b, "  source/test edit ratio: %.2f\n", metrics.EditScope.SourceToTestEditRatio)
	b.WriteString("Tool efficiency:\n")
	fmt.Fprintf(b, "  total steps: %d\n", metrics.ToolEfficiency.TotalSteps)
	fmt.Fprintf(b, "  tool calls: %d\n", metrics.ToolEfficiency.ToolCalls)
	fmt.Fprintf(b, "  search steps: %d\n", metrics.ToolEfficiency.SearchSteps)
	fmt.Fprintf(b, "  failed commands: %d\n", metrics.ToolEfficiency.FailedCommands)
	b.WriteString("Recovery:\n")
	fmt.Fprintf(b, "  recovered after failure: %s\n", yesNo(metrics.Recovery.RecoveredAfterFailure))
	fmt.Fprintf(b, "  reran tests after failure: %s\n", yesNo(metrics.Recovery.ReranTestsAfterFailure))
}

func behaviorStepDetail(step behavior.Step) string {
	for _, value := range []string{
		step.Summary,
		step.Command,
		step.Query,
		step.Target,
		strings.Join(step.Files, ", "),
		step.ToolName,
		step.Action,
	} {
		value = strings.TrimSpace(value)
		if value != "" {
			return singleLineForBehavior(value)
		}
	}
	return "-"
}

func writeJSON(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func uniqueCLIStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func singleLineForBehavior(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 160 {
		return value[:157] + "..."
	}
	return value
}
