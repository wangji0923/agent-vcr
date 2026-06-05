package list

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunsFixtureSummary(t *testing.T) {
	runs, err := Runs(fixtureProject(t))
	if err != nil {
		t.Fatalf("Runs: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("len(runs) = %d, want 2: %#v", len(runs), runs)
	}
	latest := runs[0]
	if latest.RunID != "20260604T020000Z-simple" {
		t.Fatalf("latest run = %q", latest.RunID)
	}
	if latest.Source != "codex-hooks" || latest.Status != "completed" {
		t.Fatalf("latest summary = %#v", latest)
	}
	if latest.ToolCalls != 2 || latest.ChangedFiles != 2 {
		t.Fatalf("counts = tools %d files %d", latest.ToolCalls, latest.ChangedFiles)
	}
	if latest.TestCommandSummary != "go test ./... (exit 0)" {
		t.Fatalf("test summary = %q", latest.TestCommandSummary)
	}
}

func TestRunsMissingMetadataDoesNotPanic(t *testing.T) {
	project := t.TempDir()
	runDir := filepath.Join(project, ".agent-vcr", "runs", "missing-meta")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "trace.ndjson"), []byte(`{"schema_version":"0.2","event_index":1,"type":"tool_call","source":{"adapter":"fixture"},"timestamp":"2026-06-04T00:00:00Z","payload":{"tool_name":"shell"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runs, err := Runs(project)
	if err != nil {
		t.Fatalf("Runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %#v", runs)
	}
	if !runs[0].MetadataMissing || runs[0].Source != "fixture" || runs[0].ToolCalls != 1 {
		t.Fatalf("missing metadata summary = %#v", runs[0])
	}
}

func fixtureProject(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join("..", "..", "testdata", "runs", "replay-list"))
}
