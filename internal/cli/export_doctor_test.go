package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent-vcr/agent-vcr/internal/doctor"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestExportHTMLWritesReportToOutputPath(t *testing.T) {
	project := t.TempDir()
	runID := createFixtureRun(t, project, "report", []trace.Event{
		trace.NewEvent("", trace.EventShellCommand, trace.Source{Adapter: "fixture"}),
	})
	store, err := trace.OpenRun(project, runID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.WritePatch("final.diff", []byte("diff --git a/a.go b/a.go\n+a\n")); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(project, "out", "report.html")

	stdout, _, err := executeCommand(t, "--project-dir", project, "export", runID, "--html", "--output", outputPath)
	if err != nil {
		t.Fatalf("export --html: %v", err)
	}
	if !strings.Contains(stdout, outputPath) {
		t.Fatalf("stdout = %q, want output path %q", stdout, outputPath)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	html := string(data)
	if !strings.Contains(html, "Run Summary") || !strings.Contains(html, "Final Diff") {
		t.Fatalf("report missing expected sections:\n%s", html)
	}
}

func TestExportHTMLRedactedDoesNotLeakSecret(t *testing.T) {
	project := t.TempDir()
	secret := "sk-abcdefghijklmnopqrstuvwxyz123456"
	event := trace.NewEvent("", trace.EventToolCall, trace.Source{Adapter: "fixture"})
	event.Payload = trace.Payload{"tool_name": "shell", "command": "echo " + secret, "api_key": secret}
	runID := createFixtureRun(t, project, "secret", []trace.Event{event})

	stdout, _, err := executeCommand(t, "--project-dir", project, "export", runID, "--html", "--redacted")
	if err != nil {
		t.Fatalf("export --html --redacted: %v", err)
	}
	reportPath := strings.TrimSpace(stdout)
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if strings.Contains(string(data), secret) {
		t.Fatalf("redacted export leaked secret:\n%s", string(data))
	}
	if !strings.Contains(reportPath, "-redacted") {
		t.Fatalf("report path = %q, want redacted run path", reportPath)
	}
}

func TestDoctorCommandJSONOutputIsParseable(t *testing.T) {
	project := t.TempDir()
	stdout, _, err := executeCommand(t, "--project-dir", project, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor --json: %v", err)
	}
	var result doctor.Result
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("doctor --json output is not parseable: %v\n%s", err, stdout)
	}
	if result.Cwd == "" || len(result.Checks) == 0 {
		t.Fatalf("doctor result is incomplete: %#v", result)
	}
}
