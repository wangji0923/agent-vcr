package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestDiffCommandJSONOutputIsParseable(t *testing.T) {
	project := t.TempDir()
	runA := createFixtureRun(t, project, "a", []trace.Event{
		trace.NewEvent("", trace.EventRunStart, trace.Source{Adapter: "fixture"}),
	})
	runB := createFixtureRun(t, project, "b", []trace.Event{
		trace.NewEvent("", trace.EventRunStart, trace.Source{Adapter: "fixture"}),
	})

	stdout, _, err := executeCommand(t, "--project-dir", project, "diff", runA, runB, "--json")
	if err != nil {
		t.Fatalf("diff --json returned error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("diff --json output is not parseable: %v\n%s", err, stdout)
	}
	if decoded["run_a"] != runA || decoded["run_b"] != runB {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestCheckCIExitCode(t *testing.T) {
	project := t.TempDir()
	events := []trace.Event{
		trace.NewEvent("bad", trace.EventProcessResult, trace.Source{Adapter: "fixture"}),
	}
	events[0].Payload = trace.Payload{"command": "agent", "exit_code": 0, "changed_files": []string{".env"}}
	runID := createFixtureRun(t, project, "bad", events)

	stdout, _, err := executeCommand(t, "--project-dir", project, "check", runID, "--ci")
	if err == nil {
		t.Fatal("check --ci returned nil error for failing policy")
	}
	var coder interface{ ExitCode() int }
	if !errors.As(err, &coder) || coder.ExitCode() != 1 {
		t.Fatalf("err = %v, want exit code 1", err)
	}
	if !strings.Contains(stdout, "Agent VCR check failed:") || !strings.Contains(stdout, "forbidden_paths") {
		t.Fatalf("stdout = %q", stdout)
	}
}

func createFixtureRun(t *testing.T, project string, name string, events []trace.Event) string {
	t.Helper()
	store, err := trace.CreateRun(project, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if err := store.Append(event); err != nil {
			t.Fatal(err)
		}
	}
	return store.RunID
}
