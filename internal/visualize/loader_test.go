package visualize

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/behavior"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestLoadRunsFromBehaviorCache(t *testing.T) {
	project := t.TempDir()
	runID := "run-cache"
	runDir := makeRunDir(t, project, runID)
	writeMetadata(t, runDir, metadataForRun(runID))
	writeSignature(t, runDir, signatureForSteps(runID,
		behavior.Step{RunID: runID, Kind: behavior.StepInspectTest, Files: []string{"tests/session_test.go"}, Target: "tests/session_test.go", Significant: true, SourceEventIDs: []string{"evt-read"}},
		behavior.Step{RunID: runID, Kind: behavior.StepEditFile, Files: []string{"src/session.go"}, Target: "src/session.go", Significant: true, SourceEventIDs: []string{"evt-edit"}},
	))

	runs, err := LoadRuns(context.Background(), LoadOptions{
		ProjectDir: project,
		RunIDs:     []string{runID},
		Labels:     map[string]string{runID: "cached run"},
	})
	if err != nil {
		t.Fatalf("LoadRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	run := runs[0]
	if !run.CacheHit {
		t.Fatalf("CacheHit = false, want true")
	}
	if run.Label != "cached run" {
		t.Fatalf("label = %q", run.Label)
	}
	if len(run.Timeline.Steps) != 2 || len(run.Signature.Steps) != 2 {
		t.Fatalf("steps timeline=%d signature=%d, want 2/2", len(run.Timeline.Steps), len(run.Signature.Steps))
	}

	lane := BuildLane(run, run.Label)
	if lane.RunID != runID || lane.Label != "cached run" || len(lane.Steps) != 2 {
		t.Fatalf("lane = %#v", lane)
	}
	if lane.Steps[0].Kind != VisualStepInspectTest || lane.Steps[0].Phase != VisualPhaseInspection {
		t.Fatalf("first step = %#v", lane.Steps[0])
	}
}

func TestLoadRunsRebuildsFromTraceWhenCacheMissing(t *testing.T) {
	project := t.TempDir()
	runID := "run-trace"
	runDir := makeRunDir(t, project, runID)
	writeMetadata(t, runDir, metadataForRun(runID))
	writeTrace(t, runDir,
		traceEvent(runID, "evt-1", 1, trace.EventShellCommand, trace.Payload{"command": "rg session internal"}),
		traceEvent(runID, "evt-2", 2, trace.EventShellResult, trace.Payload{"command": "rg session internal", "exit_code": 0}),
	)

	runs, err := LoadRuns(context.Background(), LoadOptions{ProjectDir: project, RunIDs: []string{runID}})
	if err != nil {
		t.Fatalf("LoadRuns: %v", err)
	}
	run := runs[0]
	if run.CacheHit {
		t.Fatalf("CacheHit = true, want false after trace rebuild")
	}
	if len(run.Signature.Steps) != 1 || run.Signature.Steps[0].Kind != behavior.StepSearch {
		t.Fatalf("signature steps = %#v", run.Signature.Steps)
	}
	if run.Metrics.ToolEfficiency.SearchSteps != 1 {
		t.Fatalf("search steps = %d, want 1", run.Metrics.ToolEfficiency.SearchSteps)
	}
	if _, err := os.Stat(behavior.SignatureCachePath(runDir)); err != nil {
		t.Fatalf("signature cache was not written: %v", err)
	}
}

func TestLoadRunsFallsBackFromCorruptCache(t *testing.T) {
	project := t.TempDir()
	runID := "run-corrupt-cache"
	runDir := makeRunDir(t, project, runID)
	writeMetadata(t, runDir, metadataForRun(runID))
	writeTrace(t, runDir, traceEvent(runID, "evt-1", 1, trace.EventFileWrite, trace.Payload{"file": "src/session.go"}))
	if err := os.MkdirAll(filepath.Dir(behavior.SignatureCachePath(runDir)), 0o755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	if err := os.WriteFile(behavior.SignatureCachePath(runDir), []byte("{"), 0o644); err != nil {
		t.Fatalf("write corrupt cache: %v", err)
	}

	runs, err := LoadRuns(context.Background(), LoadOptions{ProjectDir: project, RunIDs: []string{runID}})
	if err != nil {
		t.Fatalf("LoadRuns: %v", err)
	}
	if runs[0].CacheHit {
		t.Fatalf("CacheHit = true, want false")
	}
	if !containsText(runs[0].Warnings, "behavior cache unreadable") {
		t.Fatalf("warnings = %#v, want cache warning", runs[0].Warnings)
	}
	if len(runs[0].Signature.Steps) != 1 || runs[0].Signature.Steps[0].Kind != behavior.StepEditFile {
		t.Fatalf("signature steps = %#v", runs[0].Signature.Steps)
	}
}

func TestLoadRunsRebuildsStaleCache(t *testing.T) {
	project := t.TempDir()
	runID := "run-stale-cache"
	runDir := makeRunDir(t, project, runID)
	writeMetadata(t, runDir, metadataForRun(runID))
	writeTrace(t, runDir, traceEvent(runID, "evt-1", 1, trace.EventShellCommand, trace.Payload{"command": "go test ./..."}))
	signature := signatureForSteps(runID,
		behavior.Step{RunID: runID, Kind: behavior.StepSearch, Query: "old", Significant: true},
	)
	signature.SourceTraceHash = "sha256:stale"
	writeSignature(t, runDir, signature)

	runs, err := LoadRuns(context.Background(), LoadOptions{ProjectDir: project, RunIDs: []string{runID}})
	if err != nil {
		t.Fatalf("LoadRuns: %v", err)
	}
	if runs[0].CacheHit {
		t.Fatalf("CacheHit = true, want false")
	}
	if !containsText(runs[0].Warnings, "behavior cache stale") {
		t.Fatalf("warnings = %#v, want stale cache warning", runs[0].Warnings)
	}
	if len(runs[0].Signature.Steps) != 1 || runs[0].Signature.Steps[0].Kind != behavior.StepRunTest {
		t.Fatalf("signature steps = %#v", runs[0].Signature.Steps)
	}
}

func TestLoadRunsMissingMetadataGraceful(t *testing.T) {
	project := t.TempDir()
	runID := "run-no-meta"
	runDir := makeRunDir(t, project, runID)
	writeSignature(t, runDir, signatureForSteps(runID,
		behavior.Step{RunID: runID, Kind: behavior.StepReadFile, Files: []string{"README.md"}, Significant: true},
	))

	runs, err := LoadRuns(context.Background(), LoadOptions{ProjectDir: project, RunIDs: []string{runID}})
	if err != nil {
		t.Fatalf("LoadRuns: %v", err)
	}
	if runs[0].Metadata.Status != trace.RunStatusUnknown {
		t.Fatalf("status = %q, want unknown", runs[0].Metadata.Status)
	}
	if !containsText(runs[0].Warnings, "metadata missing") {
		t.Fatalf("warnings = %#v, want metadata warning", runs[0].Warnings)
	}
}

func TestLoadRunsClearErrors(t *testing.T) {
	project := t.TempDir()
	if _, err := LoadRuns(context.Background(), LoadOptions{ProjectDir: project, RunIDs: []string{"missing"}}); err == nil || !strings.Contains(err.Error(), "resolve visual run") {
		t.Fatalf("missing run error = %v", err)
	}

	runID := "run-bad-trace"
	runDir := makeRunDir(t, project, runID)
	writeMetadata(t, runDir, metadataForRun(runID))
	if err := os.WriteFile(filepath.Join(runDir, "trace.ndjson"), []byte("{bad json}\n"), 0o644); err != nil {
		t.Fatalf("write trace: %v", err)
	}
	_, err := LoadRuns(context.Background(), LoadOptions{ProjectDir: project, RunIDs: []string{runID}, NoCache: true})
	if err == nil || !strings.Contains(err.Error(), "parse trace line 1") {
		t.Fatalf("corrupt trace error = %v", err)
	}
}

func TestLoadRunsLabelsMaxRunsAndOrder(t *testing.T) {
	project := t.TempDir()
	for _, runID := range []string{"run-a", "run-b", "run-c"} {
		runDir := makeRunDir(t, project, runID)
		writeMetadata(t, runDir, metadataForRun(runID))
		writeSignature(t, runDir, signatureForSteps(runID,
			behavior.Step{RunID: runID, Kind: behavior.StepSearch, Query: runID, Significant: true},
		))
	}

	runs, err := LoadRuns(context.Background(), LoadOptions{
		ProjectDir: project,
		RunIDs:     []string{"run-b", "run-a"},
		Labels:     map[string]string{"run-b": "B"},
		MaxRuns:    2,
	})
	if err != nil {
		t.Fatalf("LoadRuns: %v", err)
	}
	if runs[0].RunID != "run-b" || runs[0].Label != "B" || runs[1].RunID != "run-a" {
		t.Fatalf("runs order/labels = %#v", runs)
	}

	if _, err := LoadRuns(context.Background(), LoadOptions{ProjectDir: project, RunIDs: []string{"run-a", "run-b", "run-c"}, MaxRuns: 2}); err == nil || !strings.Contains(err.Error(), "too many runs") {
		t.Fatalf("MaxRuns error = %v", err)
	}

	three, err := LoadRuns(context.Background(), LoadOptions{ProjectDir: project, RunIDs: []string{"run-a", "run-b", "run-c"}, MaxRuns: 3})
	if err != nil {
		t.Fatalf("LoadRuns three: %v", err)
	}
	report, err := BuildReportSkeleton(three, LoadOptions{MaxRuns: 3})
	if err != nil {
		t.Fatalf("BuildReportSkeleton: %v", err)
	}
	if len(report.Runs) != 3 || len(report.Lanes) != 3 || report.Mode != RenderModeCompare {
		t.Fatalf("three-run report = %#v", report)
	}
	if err := ValidateReport(&report); err != nil {
		t.Fatalf("ValidateReport: %v", err)
	}
}

func TestBuildReportSkeletonComputesFirstDivergence(t *testing.T) {
	project := t.TempDir()
	runA := "run-a"
	runB := "run-b"
	runADir := makeRunDir(t, project, runA)
	writeMetadata(t, runADir, metadataForRun(runA))
	writeSignature(t, runADir, signatureForSteps(runA,
		behavior.Step{RunID: runA, Kind: behavior.StepInspectTest, Files: []string{"tests/session_test.go"}, Target: "tests/session_test.go", Significant: true, SourceEventIDs: []string{"evt-a-read"}},
		behavior.Step{RunID: runA, Kind: behavior.StepEditFile, Files: []string{"src/session.go"}, Target: "src/session.go", Significant: true},
	))
	runBDir := makeRunDir(t, project, runB)
	writeMetadata(t, runBDir, metadataForRun(runB))
	writeSignature(t, runBDir, signatureForSteps(runB,
		behavior.Step{RunID: runB, Kind: behavior.StepReadFile, Files: []string{"src/legacy_cookie.go"}, Target: "src/legacy_cookie.go", Significant: true, SourceEventIDs: []string{"evt-b-read"}},
		behavior.Step{RunID: runB, Kind: behavior.StepEditFile, Files: []string{"src/legacy_cookie.go"}, Target: "src/legacy_cookie.go", Significant: true},
	))

	runs, err := LoadRuns(context.Background(), LoadOptions{ProjectDir: project, RunIDs: []string{runA, runB}})
	if err != nil {
		t.Fatalf("LoadRuns: %v", err)
	}
	report, err := BuildReportSkeleton(runs, LoadOptions{RunIDs: []string{runA, runB}})
	if err != nil {
		t.Fatalf("BuildReportSkeleton: %v", err)
	}
	if report.Mode != RenderModeCompare || report.Summary.FirstDivergence == nil {
		t.Fatalf("report summary = %#v", report.Summary)
	}
	if got := report.Summary.FirstDivergence.StepIndex; got != 1 {
		t.Fatalf("first divergence step = %d, want 1", got)
	}
	if len(report.Divergences) != 1 || report.Divergences[0].Kind != string(behavior.DivergenceStepChanged) {
		t.Fatalf("divergences = %#v", report.Divergences)
	}
	if !report.Lanes[0].Steps[0].Divergent || !report.Lanes[1].Steps[0].Divergent {
		t.Fatalf("divergent lane steps not marked: %#v", report.Lanes)
	}
	if err := ValidateReport(&report); err != nil {
		t.Fatalf("ValidateReport: %v", err)
	}
}

func TestVisualStepFromBehaviorStepMapsFields(t *testing.T) {
	step := behavior.Step{
		RunID:          "run-a",
		StepID:         "behavior_step_0001",
		Index:          0,
		Kind:           behavior.StepRunTest,
		Summary:        "run tests",
		Command:        "go test ./...",
		Files:          []string{`C:\repo\internal\visualize\loader_test.go`},
		Target:         `C:\repo\internal\visualize\loader_test.go`,
		Result:         behavior.ResultSuccess,
		Significant:    true,
		SourceEventIDs: []string{"evt-a"},
		SourceRefs:     []behavior.StepRef{{EventID: "evt-a"}, {EventID: "evt-b"}},
		Attributes:     map[string]string{"fingerprint_source": "test"},
	}

	visual := VisualStepFromBehaviorStep(step)
	if visual.Index != 1 || visual.Phase != VisualPhaseValidation || visual.Kind != VisualStepRunTest {
		t.Fatalf("visual step = %#v", visual)
	}
	if visual.Files[0] != "repo/internal/visualize/loader_test.go" || visual.Target != "repo/internal/visualize/loader_test.go" {
		t.Fatalf("paths files=%#v target=%q", visual.Files, visual.Target)
	}
	if len(visual.EventIDs) != 2 || visual.EventIDs[0] != "evt-a" || visual.EventIDs[1] != "evt-b" {
		t.Fatalf("event ids = %#v", visual.EventIDs)
	}
	if visual.Attributes["result"] != "success" {
		t.Fatalf("attributes = %#v", visual.Attributes)
	}
}

func makeRunDir(t *testing.T, project, runID string) string {
	t.Helper()
	runDir := filepath.Join(project, ".agent-vcr", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	return runDir
}

func metadataForRun(runID string) trace.Metadata {
	start := time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC)
	end := start.Add(time.Minute)
	return trace.Metadata{
		SchemaVersion: trace.SchemaVersion,
		RunID:         runID,
		Source:        "fixture",
		Status:        trace.RunStatusCompleted,
		Cwd:           "D:/repo",
		StartedAt:     start,
		EndedAt:       &end,
		Summary:       trace.Payload{"changed_files": []string{"src/session.go"}},
	}
}

func writeMetadata(t *testing.T, runDir string, meta trace.Metadata) {
	t.Helper()
	writeJSONFile(t, filepath.Join(runDir, "metadata.json"), meta)
}

func writeSignature(t *testing.T, runDir string, signature behavior.Signature) {
	t.Helper()
	if err := behavior.WriteSignatureCache(runDir, signature); err != nil {
		t.Fatalf("write signature: %v", err)
	}
}

func signatureForSteps(runID string, steps ...behavior.Step) behavior.Signature {
	for i := range steps {
		steps[i].RunID = runID
		steps[i].Index = i
	}
	timeline := behavior.Timeline{
		SchemaVersion: behavior.SchemaVersion,
		RunID:         runID,
		Steps:         steps,
	}
	signature := behavior.BuildSignatureFromTimeline(timeline, behavior.SignatureOptions{
		NormalizeUserPaths: true,
		IncludeSourceRefs:  true,
	})
	signature.Metrics = behavior.ComputeMetrics(timeline)
	return signature
}

func traceEvent(runID, eventID string, index int64, typ trace.EventType, payload trace.Payload) trace.Event {
	return trace.Event{
		SchemaVersion: trace.SchemaVersion,
		EventID:       eventID,
		RunID:         runID,
		EventIndex:    index,
		Type:          typ,
		Source:        trace.Source{Adapter: "fixture"},
		Timestamp:     time.Date(2026, 6, 5, 8, 0, int(index), 0, time.UTC),
		Payload:       payload,
	}
}

func writeTrace(t *testing.T, runDir string, events ...trace.Event) {
	t.Helper()
	var out strings.Builder
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("marshal event: %v", err)
		}
		out.Write(data)
		out.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(runDir, "trace.ndjson"), []byte(out.String()), 0o644); err != nil {
		t.Fatalf("write trace: %v", err)
	}
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func containsText(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
