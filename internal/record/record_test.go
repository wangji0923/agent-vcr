package record

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/agent-vcr/agent-vcr/internal/adapters/codex"
	_ "github.com/agent-vcr/agent-vcr/internal/adapters/generic"
	"github.com/agent-vcr/agent-vcr/internal/trace"
)

func TestRecordGenericCapturesStdoutStderrAndTrace(t *testing.T) {
	project := t.TempDir()
	result, err := Run(context.Background(), Options{
		ProjectDir:    project,
		Cwd:           project,
		Adapter:       "generic-cli",
		CaptureStdout: true,
		CaptureStderr: true,
		Command:       fakeRecordCommand("io"),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d", result.ExitCode)
	}
	events := readEvents(t, filepath.Join(result.RunDir, "trace.ndjson"))
	assertEventTypes(t, events, trace.EventRunStart, trace.EventProcessStart, trace.EventProcessResult, trace.EventRunStop)

	processResult := lastEventOfType(t, events, trace.EventProcessResult)
	stdoutBlob, _ := processResult.Payload["stdout_blob"].(string)
	stderrBlob, _ := processResult.Payload["stderr_blob"].(string)
	assertFileEquals(t, filepath.Join(result.RunDir, filepath.FromSlash(stdoutBlob)), "fake stdout\n")
	assertFileEquals(t, filepath.Join(result.RunDir, filepath.FromSlash(stderrBlob)), "fake stderr\n")

	envFile := filepath.Join(project, "env.txt")
	assertFileEquals(t, envFile, "generic\n"+result.RunID+"\n")
}

func TestRecordCodexJSONLStoresUnknownRawAndNormalizedEvents(t *testing.T) {
	project := t.TempDir()
	envFile := filepath.Join(project, "env.txt")
	cmd := fakeRecordCommand("jsonl")
	cmd = append(cmd, envFile)
	result, err := Run(context.Background(), Options{
		ProjectDir:    project,
		Cwd:           project,
		Adapter:       "codex-jsonl",
		CaptureStdout: true,
		CaptureStderr: true,
		Command:       cmd,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d", result.ExitCode)
	}
	events := readEvents(t, filepath.Join(result.RunDir, "trace.ndjson"))
	if countEventType(events, trace.EventRaw) == 0 {
		t.Fatalf("expected raw_event for invalid JSONL: %#v", events)
	}
	if countEventType(events, trace.EventToolCall) == 0 {
		t.Fatalf("expected normalized tool_call: %#v", events)
	}
	assertFileEquals(t, envFile, "jsonl\n"+result.RunID+"\n")
}

func TestRecordGenericCapturesFinalDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	project := t.TempDir()
	runGit(t, project, "init", "-q")
	runGit(t, project, "config", "user.email", "agent-vcr@example.test")
	runGit(t, project, "config", "user.name", "Agent VCR")
	if err := os.WriteFile(filepath.Join(project, "tracked.txt"), []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, project, "add", "tracked.txt")
	runGit(t, project, "commit", "-q", "-m", "initial")

	result, err := Run(context.Background(), Options{
		ProjectDir:    project,
		Cwd:           project,
		Adapter:       "generic-cli",
		CaptureStdout: true,
		CaptureStderr: true,
		Command:       fakeRecordCommand("write"),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d", result.ExitCode)
	}
	diff := readFile(t, filepath.Join(result.RunDir, "patches", "final.diff"))
	if !strings.Contains(diff, "+changed by fake agent") {
		t.Fatalf("final diff missing fake change:\n%s", diff)
	}
	events := readEvents(t, filepath.Join(result.RunDir, "trace.ndjson"))
	processResult := lastEventOfType(t, events, trace.EventProcessResult)
	files, ok := processResult.Payload["changed_files"].([]any)
	if !ok || len(files) == 0 || files[0] != "tracked.txt" {
		t.Fatalf("changed_files payload = %#v", processResult.Payload["changed_files"])
	}
}

func TestRecordChangedFilesUsesDelta(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tests := []struct {
		name      string
		mode      string
		wantFiles []string
	}{
		{
			name:      "preexisting dirty untouched",
			mode:      "noop",
			wantFiles: nil,
		},
		{
			name:      "new file",
			mode:      "new-file",
			wantFiles: []string{"new-agent-file.txt"},
		},
		{
			name:      "modify clean tracked file",
			mode:      "write",
			wantFiles: []string{"tracked.txt"},
		},
		{
			name:      "delete tracked file",
			mode:      "delete-tracked",
			wantFiles: []string{"tracked.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := initRecordRepoWithDirtyFile(t)
			result, err := Run(context.Background(), Options{
				ProjectDir:    project,
				Cwd:           project,
				Adapter:       "generic-cli",
				CaptureStdout: true,
				CaptureStderr: true,
				Command:       fakeRecordCommand(tt.mode),
			})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if result.ExitCode != 0 {
				t.Fatalf("exit code = %d", result.ExitCode)
			}

			events := readEvents(t, filepath.Join(result.RunDir, "trace.ndjson"))
			processResult := lastEventOfType(t, events, trace.EventProcessResult)
			assertStringSlicePayload(t, processResult.Payload["changed_files"], tt.wantFiles)

			var meta trace.Metadata
			data, err := os.ReadFile(filepath.Join(result.RunDir, "metadata.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &meta); err != nil {
				t.Fatal(err)
			}
			assertStringSlicePayload(t, meta.Summary["changed_files"], tt.wantFiles)
		})
	}
}

func TestRecordReturnsChildExitCode(t *testing.T) {
	project := t.TempDir()
	result, err := Run(context.Background(), Options{
		ProjectDir:    project,
		Cwd:           project,
		Adapter:       "generic-cli",
		CaptureStdout: true,
		CaptureStderr: true,
		Command:       fakeRecordCommand("exit"),
	})
	if err != nil {
		t.Fatalf("Run should not treat child non-zero as internal error: %v", err)
	}
	if result.ExitCode != 5 {
		t.Fatalf("exit code = %d, want 5", result.ExitCode)
	}
	events := readEvents(t, filepath.Join(result.RunDir, "trace.ndjson"))
	stop := lastEventOfType(t, events, trace.EventRunStop)
	if stop.Payload["status"] != "failed" {
		t.Fatalf("run_stop payload = %#v", stop.Payload)
	}
}

func TestFakeRecordAgent(t *testing.T) {
	mode := fakeModeFromArgs()
	switch mode {
	case "io":
		fmt.Print("fake stdout\n")
		fmt.Fprint(os.Stderr, "fake stderr\n")
		if err := os.WriteFile(filepath.Join(mustGetwd(), "env.txt"), []byte(os.Getenv("AGENT_VCR_CAPTURE_MODE")+"\n"+os.Getenv("AGENT_VCR_RUN_ID")+"\n"), 0o644); err != nil {
			panic(err)
		}
		os.Exit(0)
	case "jsonl":
		if len(os.Args) > 0 {
			envPath := os.Args[len(os.Args)-1]
			_ = os.WriteFile(envPath, []byte(os.Getenv("AGENT_VCR_CAPTURE_MODE")+"\n"+os.Getenv("AGENT_VCR_RUN_ID")+"\n"), 0o644)
		}
		_, _ = os.Stdout.Write([]byte("not-json\n" + `{"type":"item.started","item":{"type":"tool_call","name":"shell","call_id":"call-1","arguments":{"command":"go test ./..."}}}` + "\n"))
		fmt.Fprint(os.Stderr, "jsonl stderr\n")
		os.Exit(0)
	case "write":
		if err := os.WriteFile(filepath.Join(mustGetwd(), "tracked.txt"), []byte("before\nchanged by fake agent\n"), 0o644); err != nil {
			panic(err)
		}
		os.Exit(0)
	case "new-file":
		if err := os.WriteFile(filepath.Join(mustGetwd(), "new-agent-file.txt"), []byte("new by fake agent\n"), 0o644); err != nil {
			panic(err)
		}
		os.Exit(0)
	case "delete-tracked":
		if err := os.Remove(filepath.Join(mustGetwd(), "tracked.txt")); err != nil {
			panic(err)
		}
		os.Exit(0)
	case "noop":
		os.Exit(0)
	case "exit":
		fmt.Fprint(os.Stderr, "failure\n")
		os.Exit(5)
	default:
		return
	}
}

func fakeRecordCommand(mode string) []string {
	return []string{os.Args[0], "-test.run=TestFakeRecordAgent", "--", mode}
}

func fakeModeFromArgs() string {
	if mode := os.Getenv("AGENT_VCR_FAKE_RECORD"); mode != "" {
		return mode
	}
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

func mustGetwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return cwd
}

func readEvents(t *testing.T, path string) []trace.Event {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var events []trace.Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event trace.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("decode event %q: %v", scanner.Text(), err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func assertEventTypes(t *testing.T, events []trace.Event, want ...trace.EventType) {
	t.Helper()
	if len(events) < len(want) {
		t.Fatalf("events = %#v, want at least %d", events, len(want))
	}
	for i, typ := range want {
		if events[i].Type != typ {
			t.Fatalf("events[%d].Type = %s, want %s", i, events[i].Type, typ)
		}
	}
}

func countEventType(events []trace.Event, typ trace.EventType) int {
	count := 0
	for _, event := range events {
		if event.Type == typ {
			count++
		}
	}
	return count
}

func lastEventOfType(t *testing.T, events []trace.Event, typ trace.EventType) trace.Event {
	t.Helper()
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == typ {
			return events[i]
		}
	}
	t.Fatalf("missing event type %s in %#v", typ, events)
	return trace.Event{}
}

func assertFileEquals(t *testing.T, path string, want string) {
	t.Helper()
	if got := readFile(t, path); got != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runGit(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func initRecordRepoWithDirtyFile(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	runGit(t, project, "init", "-q")
	runGit(t, project, "config", "user.email", "agent-vcr@example.test")
	runGit(t, project, "config", "user.name", "Agent VCR")
	for name, content := range map[string]string{
		"tracked.txt": "before\n",
		"dirty.txt":   "clean before run\n",
	} {
		if err := os.WriteFile(filepath.Join(project, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, project, "add", "tracked.txt", "dirty.txt")
	runGit(t, project, "commit", "-q", "-m", "initial")
	if err := os.WriteFile(filepath.Join(project, "dirty.txt"), []byte("dirty before run\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return project
}

func assertStringSlicePayload(t *testing.T, value any, want []string) {
	t.Helper()
	var got []string
	switch typed := value.(type) {
	case nil:
	case []string:
		got = typed
	case []any:
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				t.Fatalf("payload item = %#v, want string", item)
			}
			got = append(got, text)
		}
	default:
		t.Fatalf("payload = %#v, want string slice", value)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("payload slice = %#v, want %#v", got, want)
	}
}
