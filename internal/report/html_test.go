package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agent-vcr/agent-vcr/internal/config"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestHTMLReportContainsSummaryTimelineAndFinalDiff(t *testing.T) {
	runDir := t.TempDir()
	writePatch(t, runDir, "final.diff", "diff --git a/src/app.go b/src/app.go\n+fmt.Println(\"ok\")\n")
	start := time.Date(2026, 6, 4, 2, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Minute)
	meta := trace.Metadata{
		RunID:        "run-report",
		Source:       "fixture",
		Status:       trace.RunStatusCompleted,
		StartedAt:    start,
		EndedAt:      &end,
		Capabilities: map[string]bool{"tools": true, "patches": true},
	}
	events := []trace.Event{
		event("evt-1", 1, trace.EventRunStart, trace.Payload{"command": "agent"}),
		event("evt-2", 2, trace.EventUserPrompt, trace.Payload{"turn_id": "turn-1", "prompt": "[REDACTED:prompt]", "prompt_sha256": "prompt-hash"}),
		event("evt-3", 3, trace.EventShellCommand, trace.Payload{"command": "go test ./...", "tool_name": "shell"}),
		event("evt-4", 4, trace.EventProcessResult, trace.Payload{"command": "go test ./...", "exit_code": 0, "changed_files": []string{"src/app.go"}}),
		event("evt-5", 5, trace.EventRunStop, trace.Payload{"turn_id": "turn-1", "last_assistant_message": "[REDACTED:assistant_message]", "last_assistant_message_sha256": "output-hash"}),
	}

	data, err := Build(runDir, meta, events, config.Default())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var out bytes.Buffer
	if err := WriteHTML(data, &out); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	html := out.String()
	for _, want := range []string{
		"Run Summary",
		"Run Input And Output",
		"prompt-hash",
		"output-hash",
		"[REDACTED:prompt]",
		"[REDACTED:assistant_message]",
		"Timeline",
		"Final Diff",
		"go test ./...",
		"src/app.go",
		"diff --git",
		"tools=true",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("report missing %q\n%s", want, html)
		}
	}
}

func TestHTMLReportEscapesUserPayload(t *testing.T) {
	runDir := t.TempDir()
	meta := trace.Metadata{RunID: "run-xss", Source: "fixture", Status: trace.RunStatusCompleted}
	events := []trace.Event{
		event("evt-xss", 1, trace.EventUserPrompt, trace.Payload{"prompt": `<script>alert(1)</script>`}),
	}
	data, err := Build(runDir, meta, events, config.Default())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var out bytes.Buffer
	if err := WriteHTML(data, &out); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	html := out.String()
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("script payload was not escaped:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;") && !strings.Contains(html, "\\u003cscript\\u003e") {
		t.Fatalf("escaped script text was not present:\n%s", html)
	}
}

func TestHTMLReportAppliesSecretRedaction(t *testing.T) {
	runDir := t.TempDir()
	secret := "sk-abcdefghijklmnopqrstuvwxyz123456"
	writePatch(t, runDir, "secret.diff", "+OPENAI_API_KEY="+secret+"\n")
	meta := trace.Metadata{RunID: "run-secret", Source: "fixture", Status: trace.RunStatusCompleted}
	events := []trace.Event{
		event("evt-prompt-secret", 1, trace.EventUserPrompt, trace.Payload{"prompt": "use " + secret, "prompt_sha256": "secret-prompt-hash"}),
		event("evt-secret", 2, trace.EventToolCall, trace.Payload{"tool_name": "shell", "api_key": secret, "command": "echo " + secret}),
	}
	data, err := Build(runDir, meta, events, config.Default())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var out bytes.Buffer
	if err := WriteHTML(data, &out); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	html := out.String()
	if strings.Contains(html, secret) {
		t.Fatalf("secret was present in report:\n%s", html)
	}
	if !strings.Contains(html, "[REDACTED") {
		t.Fatalf("redaction marker was not present:\n%s", html)
	}
}

func TestLoadMissingMetadataAndArtifactsDoesNotPanic(t *testing.T) {
	project := t.TempDir()
	runID := "run-missing"
	runDir := filepath.Join(project, ".agent-vcr", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	events := []trace.Event{event("evt-1", 1, trace.EventRaw, trace.Payload{"reason": "unknown"})}
	file, err := os.Create(filepath.Join(runDir, "trace.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, item := range events {
		if err := encoder.Encode(item); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := Load(project, runID, config.Default())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if data.Summary.Status != trace.RunStatusUnknown || data.RawEventCount != 1 {
		t.Fatalf("unexpected report data: %#v", data.Summary)
	}
	var out bytes.Buffer
	if err := WriteHTML(data, &out); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	if !strings.Contains(out.String(), "No patch artifacts found.") {
		t.Fatalf("missing patch message not found")
	}
}

func event(id string, index int64, typ trace.EventType, payload trace.Payload) trace.Event {
	return trace.Event{
		SchemaVersion: trace.SchemaVersion,
		EventID:       id,
		RunID:         "run-report",
		EventIndex:    index,
		Type:          typ,
		Source:        trace.Source{Adapter: "fixture"},
		Timestamp:     time.Date(2026, 6, 4, 2, 0, int(index), 0, time.UTC),
		Payload:       payload,
	}
}

func writePatch(t *testing.T, runDir, name, body string) {
	t.Helper()
	path := filepath.Join(runDir, "patches", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
