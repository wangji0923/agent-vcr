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

func TestRunsIgnoresUnchangedDirtySnapshot(t *testing.T) {
	project := t.TempDir()
	runDir := filepath.Join(project, ".agent-vcr", "runs", "dirty-baseline")
	writeRunFile(t, runDir, "metadata.json", `{"schema_version":"0.2","run_id":"dirty-baseline","source":"codex-hooks","status":"completed"}`)
	patch := `diff --git a/dirty.txt b/dirty.txt
--- a/dirty.txt
+++ b/dirty.txt
@@
-old
+dirty
`
	writeRunFile(t, runDir, "patches/before.patch", patch)
	writeRunFile(t, runDir, "patches/after.patch", patch)
	writeRunFile(t, runDir, "trace.ndjson", `{"schema_version":"0.2","event_index":1,"type":"tool_call","source":{"adapter":"codex-hooks"},"payload":{"tool_name":"Bash"},"artifacts":[{"kind":"patch","path":"patches/before.patch"}]}`+"\n"+
		`{"schema_version":"0.2","event_index":2,"type":"tool_result","source":{"adapter":"codex-hooks"},"payload":{"tool_name":"Bash","changed_files":["dirty.txt"]},"artifacts":[{"kind":"patch","path":"patches/after.patch"}]}`+"\n")

	runs, err := Runs(project)
	if err != nil {
		t.Fatalf("Runs: %v", err)
	}
	if len(runs) != 1 || runs[0].ChangedFiles != 0 {
		t.Fatalf("changed files = %#v, want one run with 0 delta files", runs)
	}
}

func TestRunsCountsPatchDeltaFiles(t *testing.T) {
	project := t.TempDir()
	runDir := filepath.Join(project, ".agent-vcr", "runs", "patch-delta")
	writeRunFile(t, runDir, "metadata.json", `{"schema_version":"0.2","run_id":"patch-delta","source":"codex-hooks","status":"completed"}`)
	writeRunFile(t, runDir, "patches/before.patch", `diff --git a/dirty.txt b/dirty.txt
--- a/dirty.txt
+++ b/dirty.txt
@@
-old
+dirty
`)
	writeRunFile(t, runDir, "patches/after.patch", `diff --git a/dirty.txt b/dirty.txt
--- a/dirty.txt
+++ b/dirty.txt
@@
-old
+dirty
diff --git a/src/app.go b/src/app.go
--- a/src/app.go
+++ b/src/app.go
@@
+package main
`)
	writeRunFile(t, runDir, "trace.ndjson", `{"schema_version":"0.2","event_index":1,"type":"tool_call","source":{"adapter":"codex-hooks"},"payload":{"tool_name":"Bash"},"artifacts":[{"kind":"patch","path":"patches/before.patch"}]}`+"\n"+
		`{"schema_version":"0.2","event_index":2,"type":"tool_result","source":{"adapter":"codex-hooks"},"payload":{"tool_name":"Bash","changed_files":["dirty.txt","src/app.go"]},"artifacts":[{"kind":"patch","path":"patches/after.patch"}]}`+"\n")

	runs, err := Runs(project)
	if err != nil {
		t.Fatalf("Runs: %v", err)
	}
	if len(runs) != 1 || runs[0].ChangedFiles != 1 {
		t.Fatalf("changed files = %#v, want one run with 1 delta file", runs)
	}
}

func fixtureProject(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join("..", "..", "testdata", "runs", "replay-list"))
}

func writeRunFile(t *testing.T, runDir string, relPath string, data string) {
	t.Helper()
	path := filepath.Join(runDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
