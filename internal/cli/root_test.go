package cli

import (
	"bytes"
	"strings"
	"testing"
)

func executeCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader("{}"))
	cmd.SetArgs(args)

	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand()
	if cmd == nil {
		t.Fatal("NewRootCommand() returned nil")
	}
	if got, want := cmd.Use, "agent-vcr"; got != want {
		t.Fatalf("Use = %q, want %q", got, want)
	}
}

func TestHelpDoesNotPanic(t *testing.T) {
	stdout, _, err := executeCommand(t, "--help")
	if err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	if !strings.Contains(stdout, "Behavior diff for AI coding agents.") {
		t.Fatalf("help output did not contain root description: %q", stdout)
	}
}

func TestVersionOutputNonEmpty(t *testing.T) {
	stdout, _, err := executeCommand(t, "--version")
	if err != nil {
		t.Fatalf("--version returned error: %v", err)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Fatal("--version output was empty")
	}
}

func TestExportRequiresRunID(t *testing.T) {
	_, _, err := executeCommand(t, "export", "--html")
	if err == nil {
		t.Fatal("export without run id returned nil error")
	}
}

func TestRecordRequiresCommand(t *testing.T) {
	_, _, err := executeCommand(t, "record")
	if err == nil {
		t.Fatal("record without a child command returned nil error")
	}
}

func TestHookKeepsStdoutEmptyAndReturnsSuccess(t *testing.T) {
	stdout, stderr, err := executeCommand(t, "hook")
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

func TestRegisteredCommands(t *testing.T) {
	cmd := NewRootCommand()
	expected := []string{
		"init",
		"hook",
		"record",
		"list",
		"replay",
		"diff",
		"behavior",
		"check",
		"export",
		"redact",
		"doctor",
		"version",
	}

	for _, name := range expected {
		found, _, err := cmd.Find([]string{name})
		if err != nil {
			t.Fatalf("Find(%q) returned error: %v", name, err)
		}
		if found == nil || found.Name() != name {
			t.Fatalf("command %q is not registered", name)
		}
	}
}
