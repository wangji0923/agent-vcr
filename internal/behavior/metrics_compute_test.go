package behavior

import (
	"encoding/json"
	"testing"
)

func TestMetricsReadTestsBeforeEdit(t *testing.T) {
	timeline := Timeline{Steps: []Step{
		{Kind: StepInspectTest, Files: []string{"internal/foo/foo_test.go"}},
		{Kind: StepEditFile, Files: []string{"internal/foo/foo.go"}},
	}}

	metrics := ComputeMetrics(timeline)
	if !metrics.ContextDiscipline.ReadTestsBeforeEdit {
		t.Fatalf("expected test read before edit")
	}
	if metrics.ContextDiscipline.UniqueFilesRead != 1 {
		t.Fatalf("unique files read = %d, want 1", metrics.ContextDiscipline.UniqueFilesRead)
	}
}

func TestMetricsRanTestsAfterEdit(t *testing.T) {
	timeline := Timeline{Steps: []Step{
		{Kind: StepEditFile, Files: []string{"internal/foo/foo.go"}},
		{Kind: StepRunTest, Command: "go test ./internal/foo/...", Result: ResultSuccess},
	}}

	metrics := ComputeMetrics(timeline)
	if !metrics.Validation.RanAnyTests {
		t.Fatalf("expected any tests")
	}
	if !metrics.Validation.RanTestsAfterEdit {
		t.Fatalf("expected tests after edit")
	}
}

func TestMetricsSkipValidationWhenSourceEditedWithoutTests(t *testing.T) {
	timeline := Timeline{Steps: []Step{
		{Kind: StepSearch, Query: "cookie"},
		{Kind: StepReadFile, Files: []string{"src/auth/legacy-cookie.ts"}},
		{Kind: StepEditFile, Files: []string{"src/auth/legacy-cookie.ts"}},
	}}

	report := ComputeMetricsWithOptions(timeline, MetricsOptions{})
	if !report.Metrics.ContextDiscipline.LegacyPathTouched {
		t.Fatalf("expected legacy path touched")
	}
	if report.Metrics.Validation.RanAnyTests {
		t.Fatalf("did not expect tests")
	}
	if report.Metrics.EditScope.SourceFilesEdited != 1 {
		t.Fatalf("source files edited = %d, want 1 for legacy source file", report.Metrics.EditScope.SourceFilesEdited)
	}
	if !report.Facts.SkipValidation {
		t.Fatalf("expected source edit without tests to count as skip validation")
	}
}

func TestMetricsLegacyPathTouched(t *testing.T) {
	timeline := Timeline{Steps: []Step{
		{Kind: StepReadFile, Files: []string{"legacy/parser.go"}},
	}}

	metrics := ComputeMetrics(timeline)
	if !metrics.ContextDiscipline.LegacyPathTouched {
		t.Fatalf("expected legacy path touched")
	}
}

func TestMetricsRepeatedReadsAndSearches(t *testing.T) {
	timeline := Timeline{Steps: []Step{
		{Kind: StepReadFile, Files: []string{"internal/foo/foo.go"}},
		{Kind: StepReadFile, Files: []string{"internal\\foo\\foo.go"}},
		{Kind: StepSearch, Query: "NewThing"},
		{Kind: StepSearch, Query: "NewThing"},
	}}

	report := ComputeMetricsWithOptions(timeline, MetricsOptions{})
	if report.Metrics.ContextDiscipline.UniqueFilesRead != 1 {
		t.Fatalf("unique files read = %d, want 1", report.Metrics.ContextDiscipline.UniqueFilesRead)
	}
	if report.Metrics.ContextDiscipline.RepeatedReads != 1 {
		t.Fatalf("repeated reads = %d, want 1", report.Metrics.ContextDiscipline.RepeatedReads)
	}
	if report.Facts.RepeatedSearches != 1 {
		t.Fatalf("repeated searches = %d, want 1", report.Facts.RepeatedSearches)
	}
}

func TestMetricsEditScope(t *testing.T) {
	timeline := Timeline{Steps: []Step{
		{Kind: StepEditFile, Files: []string{"internal/foo/foo.go", "internal/foo/foo.go"}},
		{Kind: StepEditFile, Files: []string{"internal/foo/foo_test.go"}},
		{Kind: StepEditFile, Files: []string{"cmd/agent-vcr/main.go"}},
	}}

	report := ComputeMetricsWithOptions(timeline, MetricsOptions{})
	if report.Metrics.EditScope.FilesEdited != 3 {
		t.Fatalf("files edited = %d, want 3", report.Metrics.EditScope.FilesEdited)
	}
	if report.Metrics.EditScope.SourceFilesEdited != 2 {
		t.Fatalf("source files edited = %d, want 2", report.Metrics.EditScope.SourceFilesEdited)
	}
	if report.Metrics.EditScope.TestFilesEdited != 1 {
		t.Fatalf("test files edited = %d, want 1", report.Metrics.EditScope.TestFilesEdited)
	}
	if report.Metrics.EditScope.SourceToTestEditRatio != 2 {
		t.Fatalf("source/test ratio = %v, want 2", report.Metrics.EditScope.SourceToTestEditRatio)
	}
	if !report.Facts.CrossUnrelatedDirectories {
		t.Fatalf("expected cross unrelated directories")
	}
}

func TestMetricsToolEfficiency(t *testing.T) {
	timeline := Timeline{Steps: []Step{
		{Kind: StepCallTool, ToolName: "Bash"},
		{Kind: StepSearch, Command: "rg TODO"},
		{Kind: StepRunBuild, Command: "go build ./cmd/agent-vcr", Result: ResultFailure},
		{Kind: StepRunTest, Command: "go test ./...", Result: ResultSuccess},
		{Kind: StepRunTest, Command: "go test ./...", Result: ResultSuccess},
	}}

	report := ComputeMetricsWithOptions(timeline, MetricsOptions{})
	if report.Metrics.ToolEfficiency.TotalSteps != 5 {
		t.Fatalf("total steps = %d, want 5", report.Metrics.ToolEfficiency.TotalSteps)
	}
	if report.Metrics.ToolEfficiency.ToolCalls != 1 {
		t.Fatalf("tool calls = %d, want 1", report.Metrics.ToolEfficiency.ToolCalls)
	}
	if report.Metrics.ToolEfficiency.SearchSteps != 1 {
		t.Fatalf("search steps = %d, want 1", report.Metrics.ToolEfficiency.SearchSteps)
	}
	if report.Metrics.ToolEfficiency.FailedCommands != 1 {
		t.Fatalf("failed commands = %d, want 1", report.Metrics.ToolEfficiency.FailedCommands)
	}
	if report.Facts.ShellCommands != 4 {
		t.Fatalf("shell commands = %d, want 4", report.Facts.ShellCommands)
	}
	if report.Facts.RepeatedCommands != 1 {
		t.Fatalf("repeated commands = %d, want 1", report.Facts.RepeatedCommands)
	}
}

func TestMetricsRecoveryAfterFailure(t *testing.T) {
	timeline := Timeline{Steps: []Step{
		{Kind: StepRunTest, Command: "go test ./...", Result: ResultFailure},
		{Kind: StepSearch, Query: "failing test name"},
		{Kind: StepEditFile, Files: []string{"internal/foo/foo.go"}},
		{Kind: StepRunTest, Command: "go test ./...", Result: ResultSuccess},
	}}

	report := ComputeMetricsWithOptions(timeline, MetricsOptions{})
	if report.Metrics.Validation.FailedTestRuns != 1 {
		t.Fatalf("failed test runs = %d, want 1", report.Metrics.Validation.FailedTestRuns)
	}
	if report.Metrics.Validation.IgnoredFailedCommand {
		t.Fatalf("did not expect ignored failed command")
	}
	if !report.Metrics.Recovery.RecoveredAfterFailure {
		t.Fatalf("expected recovered after failure")
	}
	if !report.Metrics.Recovery.ReranTestsAfterFailure {
		t.Fatalf("expected reran tests after failure")
	}
	if !report.Facts.ContinuedAfterFailure {
		t.Fatalf("expected continued after failure fact")
	}
}

func TestMetricsIgnoredAndRepeatedFailure(t *testing.T) {
	timeline := Timeline{Steps: []Step{
		{Kind: StepRunBuild, Command: "go build ./cmd/agent-vcr", Result: ResultFailure},
		{Kind: StepRunBuild, Command: "go build ./cmd/agent-vcr", Result: ResultFailure},
	}}

	report := ComputeMetricsWithOptions(timeline, MetricsOptions{})
	if !report.Metrics.Validation.IgnoredFailedCommand {
		t.Fatalf("expected ignored failed command")
	}
	if !report.Facts.RepeatedFailure {
		t.Fatalf("expected repeated failure")
	}
}

func TestMetricsSkipValidationAndMissingData(t *testing.T) {
	timeline := Timeline{Steps: []Step{
		{},
		{Kind: StepSkipValidation},
		{Kind: StepEditFile},
		{Kind: StepReadFile},
	}}

	report := ComputeMetricsWithOptions(timeline, MetricsOptions{})
	if !report.Facts.SkipValidation {
		t.Fatalf("expected skip validation fact")
	}
	if report.Metrics.ToolEfficiency.TotalSteps != 4 {
		t.Fatalf("total steps = %d, want 4", report.Metrics.ToolEfficiency.TotalSteps)
	}
	if report.Metrics.EditScope.FilesEdited != 0 {
		t.Fatalf("files edited = %d, want 0", report.Metrics.EditScope.FilesEdited)
	}
	if _, err := json.Marshal(report); err != nil {
		t.Fatalf("metrics report must be JSON serializable: %v", err)
	}
}

func TestMetricsDelta(t *testing.T) {
	before := Metrics{
		ContextDiscipline: ContextDisciplineMetrics{UniqueFilesRead: 1},
		Validation:        ValidationMetrics{RanAnyTests: false},
		EditScope:         EditScopeMetrics{FilesEdited: 1, SourceToTestEditRatio: 1},
		ToolEfficiency:    ToolEfficiencyMetrics{FailedCommands: 0},
	}
	after := Metrics{
		ContextDiscipline: ContextDisciplineMetrics{UniqueFilesRead: 3},
		Validation:        ValidationMetrics{RanAnyTests: true},
		EditScope:         EditScopeMetrics{FilesEdited: 2, SourceToTestEditRatio: 2},
		ToolEfficiency:    ToolEfficiencyMetrics{FailedCommands: 1},
	}

	delta := DiffMetrics(before, after)
	if !delta.ContextDiscipline[string(MetricUniqueFilesRead)].Changed {
		t.Fatalf("expected unique files read delta to change")
	}
	if delta.ContextDiscipline[string(MetricUniqueFilesRead)].Delta != 2 {
		t.Fatalf("unique files delta = %v, want 2", delta.ContextDiscipline[string(MetricUniqueFilesRead)].Delta)
	}
	if !delta.Validation[string(MetricRanAnyTests)].Changed {
		t.Fatalf("expected ran any tests delta to change")
	}
	if _, err := json.Marshal(delta); err != nil {
		t.Fatalf("metrics delta must be JSON serializable: %v", err)
	}
}
