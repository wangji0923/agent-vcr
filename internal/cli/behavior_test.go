package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agent-vcr/agent-vcr/internal/behavior"
)

func TestBehaviorCommandExists(t *testing.T) {
	cmd := NewRootCommand()
	found, _, err := cmd.Find([]string{"behavior"})
	if err != nil {
		t.Fatalf("Find behavior: %v", err)
	}
	if found == nil || found.Name() != "behavior" {
		t.Fatalf("behavior command not registered: %#v", found)
	}
}

func TestBehaviorLatestHumanOutput(t *testing.T) {
	stdout, _, err := executeCommand(t, "--project-dir", fixtureProject(t), "behavior", "latest", "--no-cache")
	if err != nil {
		t.Fatalf("behavior latest: %v", err)
	}
	for _, want := range []string{
		"Behavior timeline: latest",
		"run_test",
		"edit_file",
		"Metrics summary:",
		"Validation:",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("behavior output missing %q:\n%s", want, stdout)
		}
	}
}

func TestBehaviorLatestJSONOutput(t *testing.T) {
	stdout, _, err := executeCommand(t, "--project-dir", fixtureProject(t), "behavior", "latest", "--json", "--no-cache")
	if err != nil {
		t.Fatalf("behavior latest --json: %v", err)
	}
	var signature behavior.Signature
	if err := json.Unmarshal([]byte(stdout), &signature); err != nil {
		t.Fatalf("decode behavior signature JSON: %v\n%s", err, stdout)
	}
	if signature.RunID != "20260604T020000Z-simple" {
		t.Fatalf("run id = %q", signature.RunID)
	}
	if len(signature.Steps) == 0 {
		t.Fatalf("signature has no steps: %#v", signature)
	}
	if signature.Metrics.ToolEfficiency.TotalSteps == 0 {
		t.Fatalf("signature metrics not populated: %#v", signature.Metrics)
	}
}

func TestBehaviorMetricsHumanOutput(t *testing.T) {
	stdout, _, err := executeCommand(t, "--project-dir", fixtureProject(t), "behavior", "metrics", "latest", "--no-cache")
	if err != nil {
		t.Fatalf("behavior metrics: %v", err)
	}
	for _, want := range []string{
		"Context discipline:",
		"Validation:",
		"Edit scope:",
		"Tool efficiency:",
		"Recovery:",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("metrics output missing %q:\n%s", want, stdout)
		}
	}
}

func TestBehaviorMetricsJSONOutput(t *testing.T) {
	stdout, _, err := executeCommand(t, "--project-dir", fixtureProject(t), "behavior", "metrics", "latest", "--json", "--no-cache")
	if err != nil {
		t.Fatalf("behavior metrics --json: %v", err)
	}
	var report behavior.MetricsReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode metrics JSON: %v\n%s", err, stdout)
	}
	if report.Metrics.ToolEfficiency.TotalSteps == 0 {
		t.Fatalf("metrics not populated: %#v", report)
	}
}

func TestBehaviorDiffJSONOutput(t *testing.T) {
	stdout, _, err := executeCommand(t, "--project-dir", fixtureProject(t), "behavior", "diff", "20260604T010000Z-old", "latest", "--json", "--no-cache")
	if err != nil {
		t.Fatalf("behavior diff --json: %v", err)
	}
	var result behaviorDiffView
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode behavior diff JSON: %v\n%s", err, stdout)
	}
	if result.RunA != "20260604T010000Z-old" || result.RunB != "20260604T020000Z-simple" {
		t.Fatalf("diff run ids = %#v", result)
	}
	if result.FirstDivergence == nil {
		t.Fatalf("expected first divergence in diff result: %#v", result)
	}
	if result.Summary == "" {
		t.Fatalf("expected summary in diff result: %#v", result)
	}
}

func TestBehaviorRunNotFound(t *testing.T) {
	_, _, err := executeCommand(t, "--project-dir", fixtureProject(t), "behavior", "missing-run")
	if err == nil {
		t.Fatal("behavior missing-run returned nil error")
	}
	if !strings.Contains(err.Error(), "resolve behavior run") {
		t.Fatalf("unexpected error: %v", err)
	}
}
