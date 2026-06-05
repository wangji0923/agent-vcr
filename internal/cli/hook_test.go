package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestHookCodexKeepsStdoutEmptyAndReturnsSuccess(t *testing.T) {
	dir := t.TempDir()
	input := strings.ReplaceAll(string(readCLIFile(t, filepath.Join("..", "adapters", "codex", "testdata", "session_start.json"))), `"cwd": "."`, `"cwd": "`+jsonEscape(dir)+`"`)
	stdout, stderr, err := executeCommandWithInput(t, input, "--project-dir", dir, "hook", "--adapter", "codex")
	if err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	if stdout != "" {
		t.Fatalf("hook stdout = %q, want empty", stdout)
	}
	if stderr != "" {
		t.Fatalf("hook stderr = %q, want empty", stderr)
	}
}

func TestHookCodexInvalidJSONExitsZero(t *testing.T) {
	stdout, stderr, err := executeCommandWithInput(t, "{", "hook", "--adapter", "codex")
	if err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("stdout=%q stderr=%q, want both empty", stdout, stderr)
	}
}

func TestHookCodexSessionStartCreatesRun(t *testing.T) {
	dir := t.TempDir()
	input := strings.ReplaceAll(string(readCLIFile(t, filepath.Join("..", "adapters", "codex", "testdata", "session_start.json"))), `"cwd": "."`, `"cwd": "`+jsonEscape(dir)+`"`)
	if _, _, err := executeCommandWithInput(t, input, "--project-dir", dir, "hook", "--adapter", "codex"); err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	events := readLatestTraceEvents(t, dir)
	if len(events) != 1 || events[0].Type != trace.EventRunStart {
		t.Fatalf("events = %#v", events)
	}
}

func TestHookCodexCaptureModeNoOps(t *testing.T) {
	t.Setenv("AGENT_VCR_CAPTURE_MODE", "jsonl")
	dir := t.TempDir()
	input := strings.ReplaceAll(string(readCLIFile(t, filepath.Join("..", "adapters", "codex", "testdata", "session_start.json"))), `"cwd": "."`, `"cwd": "`+jsonEscape(dir)+`"`)
	stdout, stderr, err := executeCommandWithInput(t, input, "--project-dir", dir, "hook", "--adapter", "codex")
	if err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("stdout=%q stderr=%q, want both empty", stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, ".agent-vcr")); !os.IsNotExist(err) {
		t.Fatalf("capture mode hook should not create .agent-vcr, stat err = %v", err)
	}
}

func TestHookCodexUnknownEventWritesRawEvent(t *testing.T) {
	dir := t.TempDir()
	input := `{"session_id":"s_unknown","cwd":"` + jsonEscape(dir) + `","hook_event_name":"NewEvent"}`
	if _, _, err := executeCommandWithInput(t, input, "--project-dir", dir, "hook", "--adapter", "codex"); err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	events := readLatestTraceEvents(t, dir)
	if len(events) != 1 || events[0].Type != trace.EventRaw {
		t.Fatalf("events = %#v", events)
	}
	if events[0].RawRef == nil || events[0].RawRef.Kind != trace.ArtifactRaw {
		t.Fatalf("raw ref = %#v", events[0].RawRef)
	}
}

func executeCommandWithInput(t *testing.T, input string, args ...string) (string, string, error) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func readLatestTraceEvents(t *testing.T, projectDir string) []trace.Event {
	t.Helper()
	runsDir := filepath.Join(projectDir, ".agent-vcr", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no runs created")
	}
	tracePath := filepath.Join(runsDir, entries[0].Name(), "trace.ndjson")
	file, err := os.Open(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var events []trace.Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event trace.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("invalid trace line: %v", err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func readCLIFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func jsonEscape(value string) string {
	data, _ := json.Marshal(value)
	escaped := string(data)
	return escaped[1 : len(escaped)-1]
}
