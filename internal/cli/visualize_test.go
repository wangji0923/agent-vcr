package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/behavior"
	"github.com/agent-vcr/agent-vcr/internal/trace"
	"github.com/agent-vcr/agent-vcr/internal/visualize"
)

func TestVisualizeLatestUsesLatestRun(t *testing.T) {
	project := t.TempDir()
	oldRun := createVisualizeCachedRun(t, project, time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC),
		behavior.Step{Kind: behavior.StepSearch, Query: "old", Significant: true},
	)
	latestRun := createVisualizeCachedRun(t, project, time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC),
		behavior.Step{Kind: behavior.StepReadFile, Files: []string{"README.md"}, Target: "README.md", Significant: true},
	)

	stdout, _, err := executeCommand(t, "--project-dir", project, "visualize", "latest")
	if err != nil {
		t.Fatalf("visualize latest: %v", err)
	}
	if !strings.Contains(stdout, latestRun) {
		t.Fatalf("stdout = %q, want latest run %q", stdout, latestRun)
	}
	if strings.Contains(stdout, oldRun) {
		t.Fatalf("stdout = %q, did not expect old run %q", stdout, oldRun)
	}
}

func TestVisualizeJSONOutputIsParseable(t *testing.T) {
	project := t.TempDir()
	runID := createVisualizeCachedRun(t, project, time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC),
		behavior.Step{Kind: behavior.StepEditFile, Files: []string{"src/session.go"}, Target: "src/session.go", Significant: true},
	)

	stdout, _, err := executeCommand(t, "--project-dir", project, "visualize", runID, "--json")
	if err != nil {
		t.Fatalf("visualize --json: %v", err)
	}
	if strings.Contains(stdout, "Behavior visualization:") {
		t.Fatalf("--json mixed human text into stdout:\n%s", stdout)
	}
	var report visualize.VisualReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("visualize --json output is not parseable: %v\n%s", err, stdout)
	}
	if report.Runs[0].RunID != runID || report.Mode != visualize.RenderModeSingle {
		t.Fatalf("report = %#v", report)
	}
}

func TestVisualizeTwoRunsSummary(t *testing.T) {
	project := t.TempDir()
	runA := createVisualizeCachedRun(t, project, time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC),
		behavior.Step{Kind: behavior.StepInspectTest, Files: []string{"tests/session_test.go"}, Target: "tests/session_test.go", Significant: true},
		behavior.Step{Kind: behavior.StepEditFile, Files: []string{"src/session.go"}, Target: "src/session.go", Significant: true},
	)
	runB := createVisualizeCachedRun(t, project, time.Date(2026, 6, 5, 8, 1, 0, 0, time.UTC),
		behavior.Step{Kind: behavior.StepReadFile, Files: []string{"src/legacy_cookie.go"}, Target: "src/legacy_cookie.go", Significant: true},
		behavior.Step{Kind: behavior.StepEditFile, Files: []string{"src/legacy_cookie.go"}, Target: "src/legacy_cookie.go", Significant: true},
	)

	stdout, _, err := executeCommand(t, "--project-dir", project, "visualize", runA, runB)
	if err != nil {
		t.Fatalf("visualize two runs: %v", err)
	}
	for _, want := range []string{"Behavior visualization: compare", "First divergence:", runA, runB} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestVisualizeMultiRunsJSON(t *testing.T) {
	project := t.TempDir()
	runA := createVisualizeCachedRun(t, project, time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC),
		behavior.Step{Kind: behavior.StepSearch, Query: "a", Significant: true},
	)
	runB := createVisualizeCachedRun(t, project, time.Date(2026, 6, 5, 8, 1, 0, 0, time.UTC),
		behavior.Step{Kind: behavior.StepSearch, Query: "b", Significant: true},
	)
	runC := createVisualizeCachedRun(t, project, time.Date(2026, 6, 5, 8, 2, 0, 0, time.UTC),
		behavior.Step{Kind: behavior.StepSearch, Query: "c", Significant: true},
	)

	stdout, _, err := executeCommand(t, "--project-dir", project, "visualize", runA, runB, runC, "--json")
	if err != nil {
		t.Fatalf("visualize three runs --json: %v", err)
	}
	var report visualize.VisualReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	if len(report.Runs) != 3 || len(report.Lanes) != 3 || report.Mode != visualize.RenderModeCompare {
		t.Fatalf("report = %#v", report)
	}
}

func TestVisualizeHTMLWritesOutputPath(t *testing.T) {
	project := t.TempDir()
	runID := createVisualizeCachedRun(t, project, time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC),
		behavior.Step{Kind: behavior.StepRunTest, Command: "go test ./...", Significant: true},
	)
	outputPath := filepath.Join(project, "out", "visual.html")

	stdout, _, err := executeCommand(t, "--project-dir", project, "visualize", runID, "--html", "--output", outputPath)
	if err != nil {
		t.Fatalf("visualize --html: %v", err)
	}
	if !strings.Contains(stdout, outputPath) {
		t.Fatalf("stdout = %q, want output path %q", stdout, outputPath)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read html: %v", err)
	}
	html := string(data)
	for _, want := range []string{"<!doctype html>", "Swimlane Timeline", runID} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q:\n%s", want, html)
		}
	}
}

func TestVisualizeOutputWritesSummaryFile(t *testing.T) {
	project := t.TempDir()
	runID := createVisualizeCachedRun(t, project, time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC),
		behavior.Step{Kind: behavior.StepSearch, Query: "session", Significant: true},
	)
	outputPath := filepath.Join(project, "summary.txt")

	stdout, _, err := executeCommand(t, "--project-dir", project, "visualize", runID, "--output", outputPath)
	if err != nil {
		t.Fatalf("visualize --output: %v", err)
	}
	if !strings.Contains(stdout, outputPath) {
		t.Fatalf("stdout = %q, want output path %q", stdout, outputPath)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	if !strings.Contains(string(data), "Behavior visualization: single") {
		t.Fatalf("summary file = %q", string(data))
	}
}

func TestVisualizeMaxRunsLimit(t *testing.T) {
	project := t.TempDir()
	runA := createVisualizeCachedRun(t, project, time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC),
		behavior.Step{Kind: behavior.StepSearch, Query: "a"},
	)
	runB := createVisualizeCachedRun(t, project, time.Date(2026, 6, 5, 8, 1, 0, 0, time.UTC),
		behavior.Step{Kind: behavior.StepSearch, Query: "b"},
	)
	runC := createVisualizeCachedRun(t, project, time.Date(2026, 6, 5, 8, 2, 0, 0, time.UTC),
		behavior.Step{Kind: behavior.StepSearch, Query: "c"},
	)

	_, _, err := executeCommand(t, "--project-dir", project, "visualize", runA, runB, runC, "--max-runs", "2")
	if err == nil || !strings.Contains(err.Error(), "too many runs") {
		t.Fatalf("err = %v, want too many runs", err)
	}
}

func TestVisualizeMissingRunErrorIsClear(t *testing.T) {
	project := t.TempDir()
	_, _, err := executeCommand(t, "--project-dir", project, "visualize", "missing-run")
	if err == nil || !strings.Contains(err.Error(), "resolve visual run") || !strings.Contains(err.Error(), "run not found") {
		t.Fatalf("err = %v, want clear missing run error", err)
	}
}

func TestVisualizeRebuildsFromTraceWhenCacheMissing(t *testing.T) {
	project := t.TempDir()
	runID := createVisualizeTraceRun(t, project, time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC),
		trace.Payload{"command": "rg session internal"},
	)

	stdout, _, err := executeCommand(t, "--project-dir", project, "visualize", runID, "--json")
	if err != nil {
		t.Fatalf("visualize trace run --json: %v", err)
	}
	var report visualize.VisualReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	if report.Summary.StepCount == 0 || report.Runs[0].RunID != runID {
		t.Fatalf("report did not rebuild behavior from trace: %#v", report)
	}
}

func createVisualizeCachedRun(t *testing.T, project string, startedAt time.Time, steps ...behavior.Step) string {
	t.Helper()
	store, err := trace.CreateRun(project, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	writeVisualizeMetadata(t, store, startedAt)
	for i := range steps {
		steps[i].RunID = store.RunID
		steps[i].Index = i
	}
	timeline := behavior.Timeline{
		SchemaVersion: behavior.SchemaVersion,
		RunID:         store.RunID,
		Steps:         steps,
	}
	signature := behavior.BuildSignatureFromTimeline(timeline, behavior.SignatureOptions{
		IncludeSourceRefs:  true,
		NormalizeUserPaths: true,
	})
	signature.Metrics = behavior.ComputeMetrics(timeline)
	if err := behavior.WriteSignatureCache(store.RunDir, signature); err != nil {
		t.Fatalf("write signature cache: %v", err)
	}
	return store.RunID
}

func createVisualizeTraceRun(t *testing.T, project string, startedAt time.Time, payload trace.Payload) string {
	t.Helper()
	store, err := trace.CreateRun(project, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	writeVisualizeMetadata(t, store, startedAt)
	event := trace.NewEvent(store.RunID, trace.EventShellCommand, trace.Source{Adapter: "fixture"})
	event.Payload = payload
	if err := store.Append(event); err != nil {
		t.Fatalf("append trace event: %v", err)
	}
	return store.RunID
}

func writeVisualizeMetadata(t *testing.T, store *trace.Store, startedAt time.Time) {
	t.Helper()
	endedAt := startedAt.Add(time.Minute)
	meta, err := store.ReadMetadata()
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	meta.Status = trace.RunStatusCompleted
	meta.StartedAt = startedAt
	meta.EndedAt = &endedAt
	meta.Summary = trace.Payload{"changed_files": []string{"src/session.go"}}
	if err := store.WriteMetadata(meta); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
}
